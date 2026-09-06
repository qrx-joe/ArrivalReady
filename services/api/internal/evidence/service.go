package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/storage"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

// Business rules for the upload gate. Per-type caps keep an Audit's cost and
// latency bounded (TECH_SPEC §29); raising one is a product decision because
// it changes per-audit model spend.
var (
	ErrConflict = errors.New("idempotency or state conflict")

	maxImageBytes int64 = 10 << 20 // 10 MiB
	maxPDFBytes   int64 = 20 << 20 // 20 MiB
	maxReadBytes  int64 = 32 << 20 // defense-in-depth read bound

	allowedTypes = map[string]bool{"image": true, "pdf": true, "url": true, "text": true}
)

// Service wires the persistence and object-storage ports into the upload flow.
type Service struct {
	db  *store.DB
	st  storage.Store
	now func() time.Time
}

func NewService(db *store.DB, st storage.Store) *Service {
	return &Service{db: db, st: st, now: time.Now}
}

type CreateUploadInput struct {
	Type         string
	ContentType  string
	SizeBytes    int64
	JourneyStage string
	SourceNote   string
	CapturedAt   *time.Time
}

// CreateUpload validates the request, mints a private object key and returns
// a presigned PUT. The evidence row stays PENDING_UPLOAD: nothing exists in
// storage yet and nothing may flow to the AI pipeline.
func (s *Service) CreateUpload(ctx context.Context, actor auth.Actor, projectID uuid.UUID, in CreateUploadInput) (store.Evidence, storage.PresignedUpload, error) {
	if _, err := s.db.GetProject(ctx, actor, projectID); err != nil {
		// Foreign or missing project are the same error → 404 upstream.
		return store.Evidence{}, storage.PresignedUpload{}, err
	}
	if !allowedTypes[in.Type] {
		return store.Evidence{}, storage.PresignedUpload{}, fmt.Errorf("%w: unsupported evidence type %q", ErrConflict, in.Type)
	}
	limit := capFor(in.ContentType)
	if limit == 0 {
		return store.Evidence{}, storage.PresignedUpload{}, fmt.Errorf("%w: unsupported content type %q", ErrConflict, in.ContentType)
	}
	if in.SizeBytes <= 0 || in.SizeBytes > limit {
		return store.Evidence{}, storage.PresignedUpload{}, fmt.Errorf("%w: size %d outside 1..%d for %s", ErrConflict, in.SizeBytes, limit, in.ContentType)
	}

	presigned, err := s.st.NewUploadURL(ctx, actor, projectID, in.ContentType, in.SizeBytes, 15*time.Minute)
	if err != nil {
		return store.Evidence{}, storage.PresignedUpload{}, err
	}

	ev, err := s.db.CreateEvidence(ctx, actor, store.CreateEvidenceInput{
		ProjectID:    projectID,
		Type:         in.Type,
		ContentType:  in.ContentType,
		SizeBytes:    in.SizeBytes,
		JourneyStage: in.JourneyStage,
		SourceNote:   in.SourceNote,
		ObjectKey:    presigned.ObjectKey,
		CapturedAt:   in.CapturedAt,
	})
	if err != nil {
		return store.Evidence{}, storage.PresignedUpload{}, err
	}
	return ev, presigned, nil
}

// CompleteUpload is the single validation gate (B06 verify): object must
// exist, size must match, declared content-type must match the magic bytes,
// and the server-computed hash must match the client's. Any failure moves the
// row to QUARANTINED with the reason — never READY, never silently dropped.
//
// Idempotency: completing an already-READY evidence with the SAME fingerprint
// returns the stored row unchanged; a different fingerprint is a conflict.
func (s *Service) CompleteUpload(ctx context.Context, actor auth.Actor, evidenceID uuid.UUID, shaHex string, fingerprint string) (store.Evidence, error) {
	ev, err := s.db.GetEvidence(ctx, actor, evidenceID)
	if err != nil {
		return store.Evidence{}, err
	}

	switch ev.ProcessingStatus {
	case "READY", "QUARANTINED":
		if ev.SHA256 == shaHex && fingerprint == ev.Fingerprint {
			return ev, nil // exact replay of a completed upload
		}
		return store.Evidence{}, fmt.Errorf("%w: evidence %s already finalized", ErrConflict, evidenceID)
	case "DELETED":
		return store.Evidence{}, fmt.Errorf("%w: evidence %s deleted", ErrConflict, evidenceID)
	}

	reason := s.validateObject(ctx, ev, shaHex)
	if reason != "" {
		return s.db.QuarantineEvidence(ctx, actor, evidenceID, reason)
	}
	updated, err := s.db.MarkEvidenceReady(ctx, actor, evidenceID, shaHex, fingerprint)
	if err != nil {
		return store.Evidence{}, err
	}
	_ = s.db.WriteAudit(ctx, store.AuditEntry{
		ActorID:        actor.UserID,
		OrganizationID: actor.OrganizationID,
		Action:         "evidence.ready",
		ResourceType:   "evidence",
		ResourceID:     evidenceID.String(),
		After:          map[string]any{"sha256": shaHex},
	})
	return updated, nil
}

// validateObject returns "" when the upload passes every gate, otherwise the
// quarantine reason. Order matters: cheapest checks first.
func (s *Service) validateObject(ctx context.Context, ev store.Evidence, shaHex string) string {
	info, err := s.st.Stat(ctx, ev.ObjectKey)
	if err != nil {
		return "object not found: upload never completed or signature expired"
	}
	if ev.SizeBytes != nil && info.Size != *ev.SizeBytes {
		return fmt.Sprintf("size mismatch: declared %d, object %d", *ev.SizeBytes, info.Size)
	}
	detected, err := DetectMime(info.FirstBytes)
	if err != nil {
		return "unrecognized content: " + err.Error()
	}
	if err := ValidateContent(ev.MimeType, detected); err != nil {
		return err.Error()
	}

	content, err := s.st.Read(ctx, ev.ObjectKey, maxReadBytes)
	if err != nil {
		return "object unreadable or oversized: " + err.Error()
	}
	sum := sha256.Sum256(content)
	actual := hex.EncodeToString(sum[:])
	if actual != shaHex {
		return fmt.Sprintf("hash mismatch: declared %s, server computed %s", shaHex, actual)
	}
	return ""
}

// DownloadURL hands out a short-lived private URL for READY evidence only.
func (s *Service) DownloadURL(ctx context.Context, actor auth.Actor, evidenceID uuid.UUID) (string, error) {
	ev, err := s.db.GetEvidence(ctx, actor, evidenceID)
	if err != nil {
		return "", err
	}
	if ev.ProcessingStatus != "READY" {
		return "", fmt.Errorf("%w: evidence %s is %s, not READY", ErrConflict, evidenceID, ev.ProcessingStatus)
	}
	return s.st.PresignGet(ctx, ev.ObjectKey, 5*time.Minute)
}

func capFor(contentType string) int64 {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return maxImageBytes
	case "application/pdf":
		return maxPDFBytes
	default:
		return 0
	}
}

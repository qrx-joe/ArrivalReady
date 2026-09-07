package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

// Evidence mirrors the evidence table. SizeBytes/CapturedAt are optional
// because PENDING_UPLOAD rows may not know them until completion.
type Evidence struct {
	ID               uuid.UUID
	ProjectID        uuid.UUID
	OrganizationID   uuid.UUID
	Type             string
	SourceNote       string
	ObjectKey        string
	SHA256           string
	MimeType         string
	JourneyStage     string
	SizeBytes        *int64
	ProcessingStatus string
	QuarantineReason string
	Fingerprint      string
	CapturedAt       *time.Time
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
}

type CreateEvidenceInput struct {
	ProjectID    uuid.UUID
	Type         string
	ContentType  string
	SizeBytes    int64
	JourneyStage string
	SourceNote   string
	ObjectKey    string
	CapturedAt   *time.Time
}

// COALESCE keeps optional text columns scanning into plain strings; an
// unset hash/reason/fingerprint reads as "" instead of SQL NULL.
const evidenceColumns = `id, project_id, organization_id, type, source_uri_note, object_key,
	COALESCE(sha256, '') as sha256, mime_type, journey_stage, size_bytes, processing_status,
	COALESCE(quarantine_reason, '') as quarantine_reason,
	COALESCE(fingerprint, '') as fingerprint, captured_at, created_by, created_at`

func scanEvidence(row pgx.Row) (Evidence, error) {
	var e Evidence
	err := row.Scan(&e.ID, &e.ProjectID, &e.OrganizationID, &e.Type, &e.SourceNote,
		&e.ObjectKey, &e.SHA256, &e.MimeType, &e.JourneyStage, &e.SizeBytes,
		&e.ProcessingStatus, &e.QuarantineReason, &e.Fingerprint, &e.CapturedAt,
		&e.CreatedBy, &e.CreatedAt)
	return e, err
}

func (d *DB) CreateEvidence(ctx context.Context, actor auth.Actor, in CreateEvidenceInput) (Evidence, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Evidence{}, err
	}
	row := d.Pool.QueryRow(ctx, `
		INSERT INTO evidence
			(id, project_id, organization_id, type, source_uri_note, object_key,
			 mime_type, size_bytes, journey_stage, captured_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+evidenceColumns,
		id, in.ProjectID, actor.OrganizationID, in.Type, in.SourceNote, in.ObjectKey,
		in.ContentType, in.SizeBytes, stageOrDefault(in.JourneyStage), in.CapturedAt, actor.UserID)
	return scanEvidence(row)
}

// GetEvidence is org-scoped: foreign evidence is indistinguishable from
// missing (404 upstream) — the BOLA contract shared with projects.
func (d *DB) GetEvidence(ctx context.Context, actor auth.Actor, evidenceID uuid.UUID) (Evidence, error) {
	row := d.Pool.QueryRow(ctx, `
		SELECT `+evidenceColumns+` FROM evidence WHERE id = $1 AND organization_id = $2
	`, evidenceID, actor.OrganizationID)
	e, err := scanEvidence(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Evidence{}, ErrNotFound
	}
	return e, err
}

// GetEvidenceInternal loads a row WITHOUT the organization scope. It exists
// exclusively for server-side subsystems that already hold the full context
// (worker payload assembly). HTTP handlers must always use the actor-scoped
// GetEvidence — an unscoped read reachable from a request path is a BOLA hole.
func (d *DB) GetEvidenceInternal(ctx context.Context, evidenceID uuid.UUID) (Evidence, error) {
	row := d.Pool.QueryRow(ctx,
		`SELECT `+evidenceColumns+` FROM evidence WHERE id = $1`, evidenceID)
	e, err := scanEvidence(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Evidence{}, ErrNotFound
	}
	return e, err
}

// MarkEvidenceReady flips PENDING_UPLOAD → READY in one guarded UPDATE; the
// status predicate makes concurrent double-completion safe (one wins).
func (d *DB) MarkEvidenceReady(ctx context.Context, actor auth.Actor, evidenceID uuid.UUID, shaHex, fingerprint string) (Evidence, error) {
	row := d.Pool.QueryRow(ctx, `
		UPDATE evidence
		SET processing_status = 'READY', sha256 = $3, fingerprint = $4
		WHERE id = $1 AND organization_id = $2 AND processing_status = 'PENDING_UPLOAD'
		RETURNING `+evidenceColumns,
		evidenceID, actor.OrganizationID, shaHex, fingerprint)
	e, err := scanEvidence(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Evidence{}, ErrNotFound
	}
	return e, err
}

func (d *DB) QuarantineEvidence(ctx context.Context, actor auth.Actor, evidenceID uuid.UUID, reason string) (Evidence, error) {
	row := d.Pool.QueryRow(ctx, `
		UPDATE evidence
		SET processing_status = 'QUARANTINED', quarantine_reason = $3
		WHERE id = $1 AND organization_id = $2 AND processing_status = 'PENDING_UPLOAD'
		RETURNING `+evidenceColumns,
		evidenceID, actor.OrganizationID, reason)
	e, err := scanEvidence(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Evidence{}, ErrNotFound
	}
	return e, err
}

// SoftDelete keeps the row and hash (R-16): reports may explain that evidence
// is gone; nothing is physically removed here.
func (d *DB) SoftDeleteEvidence(ctx context.Context, actor auth.Actor, evidenceID uuid.UUID) error {
	tag, err := d.Pool.Exec(ctx, `
		UPDATE evidence SET processing_status = 'DELETED'
		WHERE id = $1 AND organization_id = $2 AND processing_status IN ('PENDING_UPLOAD','READY','QUARANTINED')
	`, evidenceID, actor.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) ListEvidence(ctx context.Context, actor auth.Actor, projectID uuid.UUID) ([]Evidence, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT `+evidenceColumns+` FROM evidence
		WHERE project_id = $1 AND organization_id = $2 AND processing_status <> 'DELETED'
		ORDER BY created_at DESC
	`, projectID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		e, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func stageOrDefault(stage string) string {
	if stage == "" {
		return "unspecified"
	}
	return stage
}

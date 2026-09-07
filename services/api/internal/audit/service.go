// Package audit creates and cancels audit runs (B07). The heavy lifting —
// freezing manifests, job lifecycle, result persistence — lives in store and
// worker; this service is the API-facing seam that binds the standards
// snapshot at creation time (R-03).
package audit

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/worker"
)

type Service struct {
	DB        *store.DB
	RulesPath string // resolved once at boot; the file is the 0.1.0 snapshot
}

type CreateInput struct {
	EvidenceIDs []uuid.UUID
}

// Create freezes the standards binding (version + rules hash) together with
// the evidence manifest. Every run carries exactly what it was created with,
// so activating a new standard later cannot rewrite history (R-03).
func (s *Service) Create(ctx context.Context, actor auth.Actor, projectID uuid.UUID, in CreateInput) (store.AuditRun, error) {
	version, shaHex, _, err := worker.LoadStandard(s.RulesPath)
	if err != nil {
		return store.AuditRun{}, errors.New("load standards snapshot: " + err.Error())
	}
	return s.DB.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs:     in.EvidenceIDs,
		StandardVersion: version,
		RulesSHA256:     shaHex,
	})
}

func (s *Service) Cancel(ctx context.Context, actor auth.Actor, runID uuid.UUID) (store.AuditRun, error) {
	return s.DB.CancelRun(ctx, actor, runID)
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, runID uuid.UUID) (store.AuditRun, error) {
	return s.DB.GetRun(ctx, actor, runID)
}

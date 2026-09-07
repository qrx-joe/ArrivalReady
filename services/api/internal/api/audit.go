package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/audit"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

// ---- audits (B07) ----

func (s *Server) createAudit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	if s.Audit == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit subsystem not configured (AI_SERVICE_URL empty)")
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project id")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "unreadable body")
		return
	}
	var in struct {
		EvidenceIDs []string `json:"evidence_ids"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid json")
		return
	}
	if len(in.EvidenceIDs) == 0 {
		writeProblem(w, http.StatusUnprocessableEntity, "evidence_ids must not be empty")
		return
	}
	ids := make([]uuid.UUID, 0, len(in.EvidenceIDs))
	for _, raw := range in.EvidenceIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "invalid evidence id: "+raw)
			return
		}
		ids = append(ids, id)
	}

	// create audit is one of the five idempotency-mandatory endpoints
	// (TECH_SPEC §11) — same replay/conflict pattern as upload complete.
	fp := store.Fingerprint(body)
	idemKey := r.Header.Get("Idempotency-Key")
	endpoint := "POST /projects/{projectId}/audits"
	if resp, found, err := s.DB.LookupIdempotent(r.Context(), actor.OrganizationID, idemKey, endpoint); err == nil && found {
		if resp.Fingerprint == fp {
			writeRaw(w, resp.Status, resp.Body)
			return
		}
		writeProblem(w, http.StatusConflict, "Idempotency-Key reused with different payload")
		return
	}

	run, err := s.Audit.Create(r.Context(), actor, projectID, audit.CreateInput{EvidenceIDs: ids})
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "project or evidence not found")
		return
	case err != nil:
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	out, _ := json.Marshal(envelope(runView(run), r))
	inserted, err := s.DB.SaveIdempotent(r.Context(), actor.OrganizationID, idemKey, endpoint, fp, http.StatusAccepted, out)
	if err == nil && !inserted {
		if resp, found, lerr := s.DB.LookupIdempotent(r.Context(), actor.OrganizationID, idemKey, endpoint); lerr == nil && found {
			if resp.Fingerprint == fp {
				writeRaw(w, resp.Status, resp.Body)
				return
			}
			writeProblem(w, http.StatusConflict, "Idempotency-Key reused with different payload")
			return
		}
	}
	writeRaw(w, http.StatusAccepted, out)
}

func (s *Server) listAudits(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	if s.Audit == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit subsystem not configured (AI_SERVICE_URL empty)")
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project id")
		return
	}
	runs, err := s.DB.ListRuns(r.Context(), actor, projectID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, envelope(runs, r))
}

func (s *Server) getAudit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	if s.Audit == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit subsystem not configured (AI_SERVICE_URL empty)")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "auditID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid audit id")
		return
	}
	run, err := s.Audit.Get(r.Context(), actor, id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "audit not found")
		return
	}
	findings, err := s.DB.ListFindings(r.Context(), actor, id)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "query failed")
		return
	}
	if findings == nil {
		findings = []json.RawMessage{}
	}
	writeJSON(w, http.StatusOK, envelope(map[string]any{
		"run":      runView(run),
		"findings": findings,
	}, r))
}

func (s *Server) cancelAudit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	if s.Audit == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit subsystem not configured (AI_SERVICE_URL empty)")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "auditID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid audit id")
		return
	}
	run, err := s.Audit.Cancel(r.Context(), actor, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "audit not found")
		return
	case errors.Is(err, store.ErrJobConflict):
		writeProblem(w, http.StatusConflict, "audit already in a terminal state")
		return
	case err != nil:
		writeProblem(w, http.StatusInternalServerError, "cancel failed")
		return
	}
	writeJSON(w, http.StatusOK, envelope(runView(run), r))
}

func runView(r store.AuditRun) map[string]any {
	var manifest []store.ManifestEntry
	_ = json.Unmarshal(r.EvidenceManifestJSON, &manifest)
	// Scoring (frozen at finalize) is echoed verbatim; total_score is lifted
	// to the top level per the contract's AuditRun schema.
	var scoring map[string]any
	_ = json.Unmarshal(r.ScoringJSON, &scoring)
	var totalScore any
	if scoring != nil {
		totalScore = scoring["total_score"] // nil stays nil → JSON null
	}
	return map[string]any{
		"id":                r.ID,
		"project_id":        r.ProjectID,
		"parent_run_id":     r.ParentRunID,
		"status":            r.Status,
		"status_reason":     r.StatusReason,
		"standard":          map[string]string{"code": r.StandardCode, "version": r.StandardVersion, "rules_sha256": r.RulesSHA256},
		"evidence_manifest": manifest,
		"scoring":           scoring,
		"total_score":       totalScore,
		"created_at":        r.CreatedAt,
		"completed_at":      r.CompletedAt,
	}
}

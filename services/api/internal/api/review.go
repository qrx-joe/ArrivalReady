package api

// Review, finalize, fix-task and retest endpoints (B10/B11/B12).
// Scoring follows ADR-0002 (D-018): deterministic, LLM output excluded,
// partial reports never render a verdict.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/scoring"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/worker"
)

type reviewInput struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
	Edits    struct {
		AssessmentStatus string `json:"assessment_status"`
	} `json:"edits"`
	NAReason string `json:"na_reason"`
}

func (s *Server) submitReview(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	findingID, err := uuid.Parse(chi.URLParam(r, "findingID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid finding id")
		return
	}
	var in reviewInput
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	switch in.Decision {
	case "confirm", "reject", "edit", "na":
	default:
		writeProblem(w, http.StatusUnprocessableEntity, "decision must be confirm/reject/edit/na")
		return
	}

	updated, err := s.DB.SubmitReview(r.Context(), actor, findingID, store.ReviewInput{
		Decision: in.Decision, Note: in.Note,
		NewStatus: in.Edits.AssessmentStatus, NAReason: in.NAReason,
	}, middleware.GetReqID(r.Context()))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "finding not found")
		return
	case errors.Is(err, store.ErrJobConflict):
		writeProblem(w, http.StatusConflict, "finding already reviewed (decisions are never overwritten)")
		return
	case err != nil:
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, envelope(updated, r))
}

// finalize computes the deterministic report from post-review effective
// statuses and freezes it onto the run. Missing dispositions → 422 (全部处置
// 才能出报告，ADR-0002 §2.5).
func (s *Server) finalizeAudit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	auditID, err := uuid.Parse(chi.URLParam(r, "auditID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid audit id")
		return
	}
	run, err := s.Audit.Get(r.Context(), actor, auditID)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "audit not found")
		return
	}
	if run.Status != "REVIEW_REQUIRED" {
		writeProblem(w, http.StatusConflict, "run is "+run.Status+"; only REVIEW_REQUIRED runs can be finalized")
		return
	}
	if pending, err := s.DB.PendingReviewCount(r.Context(), actor, auditID); err != nil || pending > 0 {
		writeProblem(w, http.StatusUnprocessableEntity,
			"finalize blocked: pending unreviewed findings")
		return
	}

	// Map rule codes to dimension/weight from the frozen standard file.
	_, _, std, err := worker.LoadStandard(s.RulesPath)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "standard load failed")
		return
	}
	meta := map[string]struct {
		Dim    string
		Weight float64
	}{}
	for _, rule := range std.Rules {
		meta[rule.Code] = struct {
			Dim    string
			Weight float64
		}{rule.Dimension, float64(rule.Weight)}
	}

	rows, err := s.DB.ScoringRows(r.Context(), actor, auditID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "query failed")
		return
	}
	items := make([]scoring.Item, 0, len(rows))
	blocking := []map[string]any{}
	for _, row := range rows {
		m, known := meta[row.RuleID]
		if !known {
			continue // rules outside the frozen set never score
		}
		items = append(items, scoring.Item{
			Dimension: m.Dim, Weight: m.Weight,
			Effective: row.Effective,
			Reviewed:  row.ReviewStatus != "UNREVIEWED",
		})
		if row.Effective == "FAIL" && (row.Severity == "S0" || row.Severity == "S1") {
			blocking = append(blocking, map[string]any{
				"rule_id": row.RuleID, "severity": row.Severity, "effective": row.Effective,
			})
		}
	}
	report, err := scoring.Compute(items)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	reportJSON := map[string]any{
		"dimensions":   report.Dimensions,
		"total_score":  report.TotalScore,
		"partial":      report.Partial,
		"coverage_pct": report.CoveragePct,
		"blocking":     blocking,
	}
	if err := s.DB.FinalizeRun(r.Context(), actor, auditID, mustJSON(reportJSON)); err != nil {
		writeProblem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, envelope(reportJSON, r))
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

package api

// Fix-task transitions (B11) and retest/diff (B12).

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	findingID, err := uuid.Parse(chi.URLParam(r, "findingID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid finding id")
		return
	}
	var in struct {
		WorkflowStatus string `json:"workflow_status"`
		Version        int    `json:"version"`
		Reason         string `json:"reason"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	task, err := s.DB.TransitionFixTask(r.Context(), actor, findingID,
		in.WorkflowStatus, in.Reason, middleware.GetReqID(r.Context()))
	switch {
	case errors.Is(err, store.ErrIllegalTransition):
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "finding not found")
		return
	case err != nil:
		writeProblem(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, envelope(task, r))
}

// retest: point a new run at this one as parent, reusing the SAME evidence
// set by default (显式选择来自前端上传流；此处取父 run 清单重登记为显式输入).
func (s *Server) retestAudit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	auditID, err := uuid.Parse(chi.URLParam(r, "auditID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid audit id")
		return
	}
	if s.Audit == nil {
		writeProblem(w, http.StatusServiceUnavailable, "audit subsystem not configured")
		return
	}
	parent, err := s.Audit.Get(r.Context(), actor, auditID)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "audit not found")
		return
	}
	ids := []uuid.UUID{}
	var manifest []store.ManifestEntry
	if err := json.Unmarshal(parent.EvidenceManifestJSON, &manifest); err == nil {
		for _, m := range manifest {
			ids = append(ids, m.EvidenceID)
		}
	}
	if len(ids) == 0 {
		writeProblem(w, http.StatusUnprocessableEntity, "parent run has no evidence to retest")
		return
	}
	child, err := s.DB.CreateRetestRun(r.Context(), actor, auditID, ids,
		parent.StandardVersion, parent.RulesSHA256)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "parent audit not found")
		return
	case err != nil:
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, envelope(runView(child), r))
}

func (s *Server) auditDiff(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	auditID, err := uuid.Parse(chi.URLParam(r, "auditID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid audit id")
		return
	}
	entries, note, err := s.DB.DiffAgainstParent(r.Context(), actor, auditID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "diff failed")
		return
	}
	writeJSON(w, http.StatusOK, envelope(map[string]any{"entries": entries, "note": note}, r))
}

// Package api assembles the HTTP surface (OpenAPI: contracts/openapi/arrivalready.yaml
// is the contract; keep handlers thin and push logic into domain packages).
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/evidence"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

type Server struct {
	DB       *store.DB
	Evidence *evidence.Service
}

// Routes builds the protected API. The auth middleware is applied by the
// caller (main) so liveness/readiness stay outside authentication.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Post("/projects", s.createProject)
	r.Get("/projects", s.listProjects)
	r.Get("/projects/{projectID}", s.getProject)
	r.Patch("/projects/{projectID}", s.updateProject)

	r.Post("/projects/{projectID}/evidence/upload-url", s.createUploadURL)
	// upload complete is one of the five idempotency-mandatory endpoints
	// (TECH_SPEC §11); the Idempotency-Key header is required by the contract.
	r.With(requireIdempotencyKey).Post("/projects/{projectID}/evidence", s.completeUpload)
	r.Get("/projects/{projectID}/evidence", s.listEvidence)

	r.Get("/evidence/{evidenceID}", s.getEvidence)
	r.Delete("/evidence/{evidenceID}", s.deleteEvidence)

	return r
}

func requireIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") == "" {
			writeProblem(w, http.StatusBadRequest, "Idempotency-Key header is required for this endpoint")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- projects ----

type projectInput struct {
	Name         string `json:"name"`
	EntityType   string `json:"entity_type"`
	TargetLocale string `json:"target_locale"`
	ScopeNote    string `json:"scope_note"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFrom(r.Context())
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "actor missing")
		return
	}
	var in projectInput
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Name == "" || in.EntityType == "" || in.TargetLocale == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "name, entity_type and target_locale are required")
		return
	}
	p, err := s.DB.CreateProject(r.Context(), actor, store.CreateProjectInput{
		Name: in.Name, EntityType: in.EntityType, TargetLocale: in.TargetLocale, ScopeNote: in.ScopeNote,
	})
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project: "+err.Error())
		return
	}
	_ = s.DB.WriteAudit(r.Context(), store.AuditEntry{
		ActorID: actor.UserID, OrganizationID: actor.OrganizationID,
		Action: "project.created", ResourceType: "project", ResourceID: p.ID.String(),
		After: map[string]any{"name": p.Name},
	})
	writeJSON(w, http.StatusCreated, envelope(p, r))
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	projects, err := s.DB.ListProjects(r.Context(), actor)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, envelope(projects, r))
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project id")
		return
	}
	p, err := s.DB.GetProject(r.Context(), actor, id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, envelope(p, r))
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project id")
		return
	}
	var in projectInput
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	p, err := s.DB.UpdateProjectScope(r.Context(), actor, id, in.ScopeNote)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, envelope(p, r))
}

// ---- evidence ----

type uploadURLInput struct {
	Type         string `json:"evidence_type"`
	ContentType  string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	JourneyStage string `json:"journey_stage"`
	SourceNote   string `json:"source_uri_note"`
}

func (s *Server) createUploadURL(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project id")
		return
	}
	var in uploadURLInput
	if err := decodeBody(w, r, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	ev, presigned, err := s.Evidence.CreateUpload(r.Context(), actor, projectID, evidence.CreateUploadInput{
		Type: in.Type, ContentType: in.ContentType, SizeBytes: in.SizeBytes,
		JourneyStage: in.JourneyStage, SourceNote: in.SourceNote,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, envelope(map[string]any{
		"evidence_id": ev.ID,
		"object_key":  presigned.ObjectKey,
		"upload_url":  presigned.URL,
		"expires_at":  presigned.ExpiresAt.UTC().Format(time.RFC3339),
	}, r))
}

type completeInput struct {
	EvidenceID string `json:"evidence_id"`
	ObjectKey  string `json:"object_key"`
	SHA256     string `json:"sha256"`
}

func (s *Server) completeUpload(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
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
	var in completeInput
	if err := json.Unmarshal(body, &in); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid json")
		return
	}
	evID, err := uuid.Parse(in.EvidenceID)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid evidence id")
		return
	}
	if _, err := s.DB.GetProject(r.Context(), actor, projectID); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "project not found")
		return
	}

	fp := store.Fingerprint(body)
	idemKey := r.Header.Get("Idempotency-Key")
	endpoint := "POST /projects/{projectId}/evidence"

	if resp, found, err := s.DB.LookupIdempotent(r.Context(), actor.OrganizationID, idemKey, endpoint); err == nil && found {
		if resp.Fingerprint == fp {
			writeRaw(w, resp.Status, resp.Body) // exact replay
			return
		}
		writeProblem(w, http.StatusConflict, "Idempotency-Key reused with different payload")
		return
	}

	ev, err := s.Evidence.CompleteUpload(r.Context(), actor, evID, in.SHA256, fp)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "evidence not found")
		return
	case errors.Is(err, evidence.ErrConflict):
		writeProblem(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	out, _ := json.Marshal(envelope(evidenceView(ev), r))
	inserted, err := s.DB.SaveIdempotent(r.Context(), actor.OrganizationID, idemKey, endpoint, fp, http.StatusCreated, out)
	if err == nil && !inserted {
		// Another request claimed the key while we worked: replay theirs if
		// identical, otherwise conflict.
		if resp, found, lerr := s.DB.LookupIdempotent(r.Context(), actor.OrganizationID, idemKey, endpoint); lerr == nil && found {
			if resp.Fingerprint == fp {
				writeRaw(w, resp.Status, resp.Body)
				return
			}
			writeProblem(w, http.StatusConflict, "Idempotency-Key reused with different payload")
			return
		}
	}
	writeRaw(w, http.StatusCreated, out)
}

func (s *Server) listEvidence(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid project id")
		return
	}
	list, err := s.DB.ListEvidence(r.Context(), actor, projectID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, envelope(list, r))
}

func (s *Server) getEvidence(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid evidence id")
		return
	}
	ev, err := s.DB.GetEvidence(r.Context(), actor, id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "evidence not found")
		return
	}
	view := evidenceView(ev)
	// Private, short-lived download URL for READY evidence only (B06).
	if url, err := s.Evidence.DownloadURL(r.Context(), actor, id); err == nil {
		view["download_url"] = url
	} else if ev.ProcessingStatus == "DELETED" {
		view["retention_note"] = "original object removed by retention policy; hash and metadata retained"
	}
	writeJSON(w, http.StatusOK, envelope(view, r))
}

func (s *Server) deleteEvidence(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "invalid evidence id")
		return
	}
	if err := s.DB.SoftDeleteEvidence(r.Context(), actor, id); err != nil {
		writeProblem(w, http.StatusNotFound, "evidence not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// evidenceView shapes the persistence row for the API; hash/material details
// stay visible to the owning organization only.
func evidenceView(e store.Evidence) map[string]any {
	return map[string]any{
		"id":                e.ID,
		"project_id":        e.ProjectID,
		"type":              e.Type,
		"source_uri_note":   e.SourceNote,
		"sha256":            e.SHA256,
		"mime_type":         e.MimeType,
		"journey_stage":     e.JourneyStage,
		"size_bytes":        e.SizeBytes,
		"processing_status": e.ProcessingStatus,
		"quarantine_reason": e.QuarantineReason,
		"captured_at":       e.CapturedAt,
		"created_at":        e.CreatedAt,
	}
}

// ---- helpers ----

func envelope(data any, r *http.Request) map[string]any {
	return map[string]any{"data": data, "meta": map[string]any{"request_id": middleware.GetReqID(r.Context())}}
}

func decodeBody(w http.ResponseWriter, r *http.Request, into any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return errors.New("unreadable body")
	}
	if err := json.Unmarshal(body, into); err != nil {
		return errors.New("invalid json")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeRaw(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func writeProblem(w http.ResponseWriter, code int, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  http.StatusText(code),
		"status": code,
		"detail": detail,
	})
}

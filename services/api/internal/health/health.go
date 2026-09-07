// Package health exposes liveness and readiness probes.
//
// Semantics (B04):
//   - /healthz answers 200 whenever the process is up. It must NEVER touch
//     infrastructure: orchestrators and humans use it to decide "should I
//     restart the process", and restarting cannot fix a missing database.
//   - /readyz answers 200 only when downstream dependencies are usable, and
//     503 with a machine-readable reason otherwise, so a failed dependency is
//     diagnosable from the probe response alone (TECH_SPEC §23, B04 verify).
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Pinger abstracts the dependency check so handlers stay testable without a
// real database (the concrete implementation lives in main, not here:
// interfaces belong to the consumer per TECH_SPEC §20).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves the probe endpoints. A nil Pinger means "dependency not
// configured", which is a distinct, explicitly reported state — not an error
// to hide and not a 200 to fake.
type Handler struct {
	pinger Pinger
}

func NewHandler(pinger Pinger) *Handler { return &Handler{pinger: pinger} }

// RegisterRoutes registers the probes on the caller's router (chi supports
// the same Go 1.22 method patterns); the handler type stays for tests.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.HandleFunc("GET /healthz", h.liveness)
	r.HandleFunc("GET /readyz", h.readiness)
}

func (h *Handler) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) readiness(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		writeJSON(w, http.StatusServiceUnavailable, probeState{
			Status: "unavailable",
			Reason: "database not configured (DATABASE_URL is empty)",
		})
		return
	}

	// A probe must fail fast: a hung database should flip readiness quickly
	// rather than pile up stuck connections.
	ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, probeState{
			Status: "unavailable",
			Reason: "postgres unreachable: " + err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, probeState{Status: "ready"})
}

const probeTimeout = 2 * time.Second

type probeState struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

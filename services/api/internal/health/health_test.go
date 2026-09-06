package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestLivenessAlwaysOK(t *testing.T) {
	// Liveness must not depend on infrastructure: even a nil Pinger (no DB
	// configured) answers 200, because a restart cannot fix configuration.
	h := NewHandler(nil)
	rec := httptest.NewRecorder()
	h.liveness(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness with nil pinger: got %d, want 200", rec.Code)
	}
}

func TestReadinessNotConfigured(t *testing.T) {
	h := NewHandler(nil)
	rec := httptest.NewRecorder()
	h.readiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness without DATABASE_URL: got %d, want 503", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "DATABASE_URL") {
		t.Fatalf("readiness body must name the missing config, got: %s", body)
	}
}

func TestReadinessDBUnreachable(t *testing.T) {
	h := NewHandler(fakePinger{err: errors.New("connection refused")})
	rec := httptest.NewRecorder()
	h.readiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness with failing ping: got %d, want 503", rec.Code)
	}
}

func TestReadinessReady(t *testing.T) {
	h := NewHandler(fakePinger{err: nil})
	rec := httptest.NewRecorder()
	h.readiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readiness with healthy ping: got %d, want 200", rec.Code)
	}
}

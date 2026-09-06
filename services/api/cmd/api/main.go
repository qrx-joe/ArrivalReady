// Command api is the Arrival Ready application API entrypoint (Go side of the
// modular monolith, TECH_SPEC §3.2). It owns auth, business state machines and
// persistence; the AI service is a downstream worker it calls, never the other
// way around (execution plan §5.2).
//
// Boot contract: the process starts even with degraded configuration so that
// liveness stays truthful; readiness and protected routes report the exact
// missing dependency. There is deliberately NO authenticator fallback: with
// auth unconfigured, protected routes answer 503 (fail closed), and the local
// test identity cannot exist outside ENV=dev (B05).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/config"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/health"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

func main() {
	// JSON slog from the start: structured logs are a B05 requirement
	// (request_id/actor fields) and cost nothing to adopt now.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		// Surface the full problem list, then keep booting: liveness is
		// process truth, readiness is dependency truth.
		logger.Warn("configuration incomplete", "problems", cfgErr.Error())
	}

	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		// pgxpool.New does not connect eagerly; /readyz drives the first real
		// ping so boot never blocks on infrastructure.
		p, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
		if err != nil {
			logger.Error("invalid DATABASE_URL", "error", err)
			os.Exit(1)
		}
		defer p.Close()
		pool = p
	}

	authn := buildAuthenticator(cfg, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Mount("/", health.NewHandler(pool).Routes())

	// Protected routes fail closed: without a database there is no user
	// lookup and without an authenticator there is no identity — both are 503
	// with a reason, never an implicit bypass.
	if authn != nil && pool != nil {
		db := &store.DB{Pool: pool}
		r.Group(func(pr chi.Router) {
			pr.Use(auth.Middleware(authn, db))
			pr.Get("/projects", func(w http.ResponseWriter, r *http.Request) {
				actor, ok := auth.ActorFrom(r.Context())
				if !ok {
					writeProblem(w, http.StatusInternalServerError, "actor missing from context")
					return
				}
				projects, err := db.ListProjects(r.Context(), actor)
				if err != nil {
					logger.Error("list projects", "error", err)
					writeProblem(w, http.StatusInternalServerError, "query failed")
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"data": projects,
					"meta": map[string]any{"request_id": middleware.GetReqID(r.Context())},
				})
			})
		})
	} else {
		reason := "service degraded: authenticator or database not configured"
		r.Get("/projects", func(w http.ResponseWriter, _ *http.Request) {
			writeProblem(w, http.StatusServiceUnavailable, reason)
		})
	}

	// Dependency-free 404 in the API error shape so clients always get JSON.
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Not Found","status":404}`))
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api listening", "port", cfg.Port, "env", cfg.Env, "auth", cfg.AuthMode())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

// buildAuthenticator wires the identity source. Order matters: the local test
// identity is only ever an explicit dev opt-in, never a fallback when OIDC is
// missing (B05 step ⑤; a misconfigured gate aborts the process loudly).
func buildAuthenticator(cfg config.Config, logger *slog.Logger) auth.Authenticator {
	if cfg.Env == "dev" && cfg.TestIdentityEnabled == "1" {
		authn, err := auth.NewTestAuthenticator(auth.TestIdentityConfig{
			Env:        cfg.Env,
			EnableFlag: cfg.TestIdentityEnabled,
			Secret:     cfg.TestIdentitySecret,
		})
		if err != nil {
			logger.Error("test identity requested but refused", "error", err)
			os.Exit(1)
		}
		logger.Warn("USING LOCAL TEST IDENTITY — never acceptable with real data", "env", cfg.Env)
		return authn
	}
	if cfg.OIDCIssuer != "" && cfg.OIDCAudience != "" && cfg.OIDCJWKSURL != "" {
		return auth.NewOIDCAuthenticator(cfg.OIDCIssuer, cfg.OIDCAudience, cfg.OIDCJWKSURL, nil)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
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

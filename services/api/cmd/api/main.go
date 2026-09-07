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

	"github.com/qrx-joe/ArrivalReady/services/api/internal/api"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/audit"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/config"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/evidence"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/health"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/storage"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/worker"
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
	objectStore := buildObjectStore(cfg, logger)
	auditSvc, workerSvc := buildAudit(cfg, logger, pool, objectStore)

	var db *store.DB
	var evidenceSvc *evidence.Service
	if pool != nil && objectStore != nil {
		db = &store.DB{Pool: pool}
		evidenceSvc = evidence.NewService(db, objectStore)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Mount("/", health.NewHandler(pool).Routes())

	// Protected routes fail closed: without a database there is no user
	// lookup, without an authenticator there is no identity, and without
	// object storage uploads cannot validate — 503 with a reason, never a
	// silent bypass.
	if authn != nil && db != nil && evidenceSvc != nil {
		server := &api.Server{DB: db, Evidence: evidenceSvc, Audit: auditSvc}
		r.Group(func(pr chi.Router) {
			pr.Use(auth.Middleware(authn, db))
			pr.Mount("/", server.Routes())
		})
	} else {
		r.HandleFunc("/*", func(w http.ResponseWriter, _ *http.Request) {
			writeProblem(w, http.StatusServiceUnavailable,
				"service degraded: authenticator, database or object storage not configured")
		})
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The worker starts only with a full stack; a degraded API still serves
	// liveness/readiness but never half-runs jobs.
	if workerSvc != nil {
		go workerSvc.Run(ctx)
	}

	go func() {
		logger.Info("api listening", "port", cfg.Port, "env", cfg.Env,
			"auth", cfg.AuthMode(), "storage", cfg.StorageMode())
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

// buildAudit wires the audit service and, when the full stack exists (DB +
// AI service URL + bound rules file), the background assessment worker.
func buildAudit(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, objectStore storage.Store) (*audit.Service, *worker.Worker) {
	if cfg.AIServiceURL == "" || pool == nil {
		logger.Warn("audit subsystem not configured (AI_SERVICE_URL or DATABASE_URL empty)")
		return nil, nil
	}
	rulesPath, err := worker.ResolveStandardsPath(cfg.StandardsRulesPath)
	if err != nil {
		logger.Error("standards rules file not found", "error", err)
		os.Exit(1)
	}
	svc := &audit.Service{DB: &store.DB{Pool: pool}, RulesPath: rulesPath}
	w := &worker.Worker{
		DB:           svc.DB,
		AI:           &worker.HTTPAIClient{BaseURL: cfg.AIServiceURL},
		RulesPath:    rulesPath,
		Storage:      objectStore,
		BuildPayload: worker.BuildAuditPayload(svc.DB, objectStore, rulesPath),
	}
	return svc, w
}

// buildObjectStore wires S3-compatible storage. Missing config is a logged
// warning + nil: uploads answer 503 instead of pretending to work.
func buildObjectStore(cfg config.Config, logger *slog.Logger) storage.Store {
	if cfg.S3Endpoint == "" || cfg.S3Bucket == "" {
		logger.Warn("object storage not configured (S3_ENDPOINT / S3_BUCKET empty)")
		return nil
	}
	st, err := storage.NewMinIOStore(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	if err != nil {
		logger.Error("object storage client failed", "error", err)
		os.Exit(1)
	}
	return st
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

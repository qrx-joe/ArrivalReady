// Command api is the Arrival Ready application API entrypoint (Go side of the
// modular monolith, TECH_SPEC §3.2). It owns auth, business state machines and
// persistence; the AI service is a downstream worker it calls, never the other
// way around (execution plan §5.2).
//
// Boot contract: the process starts even with degraded configuration so that
// liveness stays truthful; readiness reports the exact missing dependency.
package main

import (
	"context"
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

	"github.com/qrx-joe/ArrivalReady/services/api/internal/config"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/health"
)

func main() {
	// JSON slog from the start: structured logs are a B05+ requirement
	// (request_id/trace_id fields) and cost nothing to adopt now.
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

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Mount("/", health.NewHandler(pool).Routes())

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
		logger.Info("api listening", "port", cfg.Port, "env", cfg.Env)
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

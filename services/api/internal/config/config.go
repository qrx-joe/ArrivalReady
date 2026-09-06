// Package config loads service configuration from the environment.
//
// Contract:
//   - Missing required-for-role configuration is surfaced as an explicit,
//     per-variable error list. The API still BOOTS with a missing DATABASE_URL
//     (liveness must not depend on infrastructure); readiness reports the gap
//     instead of silently answering 200 (B04 verify: 缺配置能明确报错).
//   - No secrets are ever logged; callers log summaries, not raw values.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config carries everything the API process needs at boot.
type Config struct {
	// Port is the HTTP listen port (PORT, default "8080").
	Port string
	// DatabaseURL is the PostgreSQL DSN (DATABASE_URL). Empty means "not
	// configured": the process stays alive and /readyz reports 503 with a
	// diagnostic reason until it is provided.
	DatabaseURL string
	// AIServiceURL is the internal AI service base URL (AI_SERVICE_URL).
	// Empty is tolerated at this stage; audit jobs (B07) will hard-require it.
	AIServiceURL string
	// Env selects log verbosity (ENV: "dev" or "production").
	Env string
}

// Load reads the environment and reports every missing-but-expected variable
// at once, so a misconfigured environment is fixable in one round trip.
func Load() (Config, error) {
	var problems []string

	cfg := Config{
		Port:         envOr("PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AIServiceURL: os.Getenv("AI_SERVICE_URL"),
		Env:          envOr("ENV", "dev"),
	}

	if strings.TrimSpace(cfg.Port) == "" {
		problems = append(problems, "PORT is set but empty")
	}
	if v := os.Getenv("DATABASE_URL"); v == "" {
		problems = append(problems,
			"DATABASE_URL is not set: /readyz will report 503 until PostgreSQL is configured")
	}
	if v := os.Getenv("ENV"); v != "" && v != "dev" && v != "production" {
		problems = append(problems, fmt.Sprintf("ENV must be \"dev\" or \"production\", got %q", v))
	}

	if len(problems) > 0 {
		return cfg, errors.New("configuration problems:\n  - " + strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

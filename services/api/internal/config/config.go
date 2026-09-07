// Package config loads service configuration from the environment.
//
// Contract:
//   - Missing required-for-role configuration is surfaced as an explicit,
//     per-variable error list. The API still BOOTS with degraded config
//     (liveness must not depend on infrastructure); readiness and protected
//     routes report the gap instead of silently answering 200 (B04/B05).
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
	// Env selects log verbosity and identity gates (ENV: "dev" or "production").
	Env string

	// OIDC settings (B05). All three are required to verify real tokens.
	OIDCIssuer   string
	OIDCAudience string
	OIDCJWKSURL  string

	// StandardsRulesPath overrides the bound rules.yaml (default: repo layout).
	StandardsRulesPath string

	// S3-compatible object storage (B06). Local dev: MinIO from compose.
	S3Endpoint  string // e.g. localhost:9000
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool

	// TestIdentity gates (auth.TestIdentityConfig). Secret is sensitive:
	// consume it only through code, never log it.
	TestIdentityEnabled string
	TestIdentitySecret  string
}

// StorageMode reports whether object storage is configured.
func (c Config) StorageMode() string {
	if c.S3Endpoint != "" && c.S3Bucket != "" {
		return "s3"
	}
	return "unconfigured"
}

// AuthMode reports which authenticator the process will install, so main can
// decide fail-closed behaviour without duplicating the gate logic.
func (c Config) AuthMode() string {
	switch {
	case c.Env == "dev" && c.TestIdentityEnabled == "1":
		return "test-identity"
	case c.OIDCIssuer != "" && c.OIDCAudience != "" && c.OIDCJWKSURL != "":
		return "oidc"
	default:
		return "unconfigured"
	}
}

// Load reads the environment and reports every missing-but-expected variable
// at once, so a misconfigured environment is fixable in one round trip.
func Load() (Config, error) {
	var problems []string

	cfg := Config{
		Port:                envOr("PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		AIServiceURL:        os.Getenv("AI_SERVICE_URL"),
		Env:                 envOr("ENV", "dev"),
		OIDCIssuer:          os.Getenv("OIDC_ISSUER"),
		OIDCAudience:        os.Getenv("OIDC_AUDIENCE"),
		OIDCJWKSURL:         os.Getenv("OIDC_JWKS_URL"),
		StandardsRulesPath:  os.Getenv("STANDARDS_RULES_PATH"),
		S3Endpoint:          os.Getenv("S3_ENDPOINT"),
		S3Bucket:            envOr("S3_BUCKET", "arrivalready-evidence"),
		S3AccessKey:         os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:         os.Getenv("S3_SECRET_KEY"),
		S3UseSSL:            os.Getenv("S3_USE_SSL") == "1",
		TestIdentityEnabled: os.Getenv("ARRIVAL_ENABLE_TEST_IDENTITY"),
		TestIdentitySecret:  os.Getenv("ARRIVAL_TEST_IDENTITY_SECRET"),
	}

	if strings.TrimSpace(cfg.Port) == "" {
		problems = append(problems, "PORT is set but empty")
	}
	if cfg.DatabaseURL == "" {
		problems = append(problems,
			"DATABASE_URL is not set: /readyz will report 503 until PostgreSQL is configured")
	}
	if v := os.Getenv("ENV"); v != "" && v != "dev" && v != "production" {
		problems = append(problems, fmt.Sprintf("ENV must be \"dev\" or \"production\", got %q", v))
	}
	if cfg.AuthMode() == "unconfigured" {
		problems = append(problems,
			"no authenticator available: set OIDC_ISSUER/OIDC_AUDIENCE/OIDC_JWKS_URL "+
				"(the local test identity is dev-only and cannot be a production fallback)")
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

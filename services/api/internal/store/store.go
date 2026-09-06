// Package store is the persistence layer (pgx-based repositories).
//
// Tenancy invariant (execution plan B05 / OWASP API1): every project query
// carries the actor's organization_id as a WHERE condition — the caller's
// actor comes from the auth middleware, never from request payloads. A
// repository that accepts an organization id from the caller instead of the
// authenticated actor is a bug; keep the two parameters visibly distinct.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	// migrate's postgres driver matches the postgres:// URL scheme and brings
	// its own database/sql wiring; the file source reads database/migrations.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a scoped lookup matches no row. Callers must
// NOT distinguish "wrong tenant" from "does not exist" in API responses —
// both surface as 404 to prevent cross-tenant resource enumeration.
var ErrNotFound = errors.New("resource not found")

// DB wraps the connection pool. Repositories hang off it; keep this the only
// place that imports pgxpool so pool lifecycle stays in main.
type DB struct {
	Pool *pgxpool.Pool
}

// MigrationsDir resolves the SQL migrations directory. Lookup order: explicit
// override (ARRIVAL_MIGRATIONS_DIR), repository-root relative (used by tests
// and `go run` from services/api), cwd relative (used by cmd/migrate from the
// repository root). Explicit > convention, so containers can mount anywhere.
func MigrationsDir() (string, error) {
	if v := os.Getenv("ARRIVAL_MIGRATIONS_DIR"); v != "" {
		if ok, err := existsDir(v); ok && err == nil {
			return v, nil
		}
		return "", fmt.Errorf("ARRIVAL_MIGRATIONS_DIR=%s does not exist", v)
	}
	candidates := []string{
		filepath.Join("..", "..", "database", "migrations"),             // run from services/api
		filepath.Join("..", "..", "..", "..", "database", "migrations"), // tests run from services/api/internal/store
		filepath.Join(".", "database", "migrations"),                    // run from repo root
	}
	for _, c := range candidates {
		if ok, err := existsDir(c); ok && err == nil {
			return c, nil
		}
	}
	return "", errors.New(
		"migrations directory not found: set ARRIVAL_MIGRATIONS_DIR or run from the repository layout")
}

func existsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// MigrateUp applies all pending migrations. Used by tests and cmd/migrate;
// the API server never migrates on boot (production safety, ADR-0003).
func MigrateUp(databaseURL, migrationsDir string) error {
	m, err := newMigrator(databaseURL, migrationsDir)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateDownAll reverts every migration. ONLY for throwaway test databases
// and rollback rehearsal — the down path on a real database is destructive
// and requires backup + ADR (execution plan §7.3).
func MigrateDownAll(databaseURL, migrationsDir string) error {
	m, err := newMigrator(databaseURL, migrationsDir)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateVersion reports the currently applied schema version and dirty flag.
func MigrateVersion(databaseURL, migrationsDir string) (uint, bool, error) {
	m, err := newMigrator(databaseURL, migrationsDir)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	return m.Version()
}

func newMigrator(databaseURL, migrationsDir string) (*migrate.Migrate, error) {
	sourceURL := "file://" + filepath.ToSlash(migrationsDir)
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open migrations: %w", err)
	}
	return m, nil
}

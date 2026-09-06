// Package testutil provides throwaway-environment helpers shared by
// integration tests. Everything here is for TESTS ONLY: it migrates real
// databases and creates real buckets, so it must never be imported by
// production code.
//
// It deliberately re-implements the tiny migrate call instead of importing
// internal/store: store's own tests import testutil, and a store→testutil→store
// edge is an import cycle in Go's test compilation model.
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

var migrateOnce sync.Once

// ConnectPG opens a pool to a throwaway PostgreSQL (TEST_DATABASE_URL).
func ConnectPG(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// MigrationsDir mirrors store.MigrationsDir's lookup for test binaries whose
// working directory is the package folder under services/api.
func MigrationsDir(t *testing.T) string {
	t.Helper()
	for _, c := range []string{
		filepath.Join("..", "..", "database", "migrations"),
		filepath.Join("..", "..", "..", "..", "database", "migrations"),
		filepath.Join(".", "database", "migrations"),
	} {
		if info, err := filepath.Abs(c); err == nil {
			if dirExists(info) {
				return info
			}
		}
	}
	t.Fatal("migrations directory not found; set ARRIVAL_MIGRATIONS_DIR or keep the repository layout")
	return ""
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// MigrateTestDB applies all migrations exactly once per process. The full
// up→down→up cycle belongs to store's dedicated TestMigrationUpDownCycle.
func MigrateTestDB(t *testing.T, dsn string) {
	t.Helper()
	dir := MigrationsDir(t)
	migrateOnce.Do(func() {
		m, err := migrate.New("file://"+filepath.ToSlash(dir), dsn)
		if err != nil {
			t.Fatalf("open migrations: %v", err)
		}
		defer m.Close()
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			t.Fatalf("migrate up: %v", err)
		}
	})
}

// SeedOrgUser inserts one organization and one active user, returning the
// actor the auth middleware would have produced. Takes the raw pool so test
// packages (store, evidence) can use it without an import cycle.
func SeedOrgUser(t *testing.T, pool *pgxpool.Pool, email string) auth.Actor {
	t.Helper()
	ctx := context.Background()
	orgID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	userID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name) VALUES ($1, $2)`, orgID, "org-"+email); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, organization_id, role, email) VALUES ($1, $2, 'MEMBER', $3)`,
		userID, orgID, email); err != nil {
		t.Fatal(err)
	}
	return auth.Actor{UserID: userID, OrganizationID: orgID, Role: "MEMBER", Email: email}
}

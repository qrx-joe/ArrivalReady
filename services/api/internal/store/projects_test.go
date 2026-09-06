package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/testutil"
)

// Integration tests run against a REAL ephemeral PostgreSQL (execution plan
// §6 matrix: 单测 + 真实临时 PostgreSQL 集成 + migration 空库/升级检查).
//
// TEST_DATABASE_URL must point at a THROWAWAY database: tests migrate the
// schema and seed rows. Without the variable the suite skips loudly; CI
// always provides a postgres service container.
func testDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set: integration suite requires a throwaway PostgreSQL (see header)")
	}
	pool := testutil.ConnectPG(t, dsn)
	testutil.MigrateTestDB(t, dsn)
	return &DB{Pool: pool}
}

// TestMigrationUpDownCycle is the 空库/升级检查: every up must apply cleanly,
// every down must reverse cleanly, and a second up must reach the same shape.
// It deliberately reverts and re-applies (throwaway DB only).
func TestMigrationUpDownCycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	dir, err := MigrationsDir()
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	if err := MigrateUp(dsn, dir); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := MigrateDownAll(dsn, dir); err != nil {
		t.Fatalf("down all: %v", err)
	}
	if err := MigrateUp(dsn, dir); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestOrganizationIsolation(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	alice := testutil.SeedOrgUser(t, d.Pool, "alice@org-a.test")
	bob := testutil.SeedOrgUser(t, d.Pool, "bob@org-b.test")

	created, err := d.CreateProject(ctx, alice, CreateProjectInput{
		Name: "Org A 门店", EntityType: "restaurant", TargetLocale: "en-US",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Owner org sees exactly its project.
	listA, err := d.ListProjects(ctx, alice)
	if err != nil || len(listA) != 1 {
		t.Fatalf("org A list: got %d projects, err=%v", len(listA), err)
	}

	// Foreign org sees nothing — list, get and update all fail closed.
	listB, err := d.ListProjects(ctx, bob)
	if err != nil || len(listB) != 0 {
		t.Fatalf("org B list must be empty, got %d, err=%v", len(listB), err)
	}
	if _, err := d.GetProject(ctx, bob, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org B get must be ErrNotFound (indistinguishable from missing), got %v", err)
	}
	if _, err := d.UpdateProjectScope(ctx, bob, created.ID, "hijack"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org B update must be ErrNotFound, got %v", err)
	}

	// The row is untouched by the failed cross-tenant attempts.
	after, err := d.GetProject(ctx, alice, created.ID)
	if err != nil || after.ScopeNote != "" || after.Version != 1 {
		t.Fatalf("project mutated by cross-org attempt: %+v err=%v", after, err)
	}

	// WriteAudit path works with a real table and actor.
	if err := d.WriteAudit(ctx, AuditEntry{
		ActorID: alice.UserID, OrganizationID: alice.OrganizationID,
		Action: "project.created", ResourceType: "project", ResourceID: created.ID.String(),
		RequestID: "req_test_1", After: map[string]any{"name": created.Name},
	}); err != nil {
		t.Fatalf("audit write: %v", err)
	}
	var count int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE resource_id = $1`, created.ID.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("audit rows: got %d, want 1", count)
	}
}

package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

// Integration tests run against a REAL ephemeral PostgreSQL (execution plan
// §6 matrix: 单测 + 真实临时 PostgreSQL 集成 + migration 空库/升级检查).
//
// Required environment:
//
//	TEST_DATABASE_URL   postgres DSN of a THROWAWAY database (CI service
//	                    container, or scripts' initdb cluster). Tests migrate
//	                    up AND down against it; never point this at a real
//	                    database.
//
// Without it the suite skips with a loud notice; CI always provides the
// variable so the skip can never silently replace verification.
func testDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set: integration suite requires a throwaway PostgreSQL (see header)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	dir, err := MigrationsDir()
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	// 空库/升级检查: full up, then down all, then up again — proves both
	// directions and that a fresh environment reaches the same shape.
	if err := MigrateUp(dsn, dir); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if err := MigrateDownAll(dsn, dir); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	if err := MigrateUp(dsn, dir); err != nil {
		t.Fatalf("migrate up (second): %v", err)
	}
	return &DB{Pool: pool}
}

func orgActor(t *testing.T, d *DB, email, role string) auth.Actor {
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
	if _, err := d.Pool.Exec(ctx,
		`INSERT INTO organizations (id, name) VALUES ($1, $2)`, orgID, "org-"+email); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx,
		`INSERT INTO users (id, organization_id, role, email) VALUES ($1, $2, $3, $4)`,
		userID, orgID, role, email); err != nil {
		t.Fatal(err)
	}
	return auth.Actor{UserID: userID, OrganizationID: orgID, Role: role, Email: email}
}

func TestOrganizationIsolation(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	alice := orgActor(t, d, "alice@org-a.test", "MEMBER")
	bob := orgActor(t, d, "bob@org-b.test", "MEMBER")

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

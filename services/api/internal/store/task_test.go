package store

// Failure-path tests for the fix-task state machine (state-machines.md §4):
//   - stale/mismatched version must conflict (optimistic lock);
//   - READY_FOR_RETEST→RESOLVED without a human-confirmed PASS in a child
//     retest run must be refused — clicking through the UI cannot manufacture
//     a resolution (R-06/R-07).

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/testutil"
)

// seedFindingForTask inserts a minimal COMPLETED run + reviewed finding so a
// task can be exercised without the AI worker.
func seedFindingForTask(t *testing.T, d *DB, actor auth.Actor) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	runID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	findingID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	project, err := d.CreateProject(ctx, actor, CreateProjectInput{
		Name: "task-guard " + runID.String()[:8], EntityType: "restaurant", TargetLocale: "en-US",
	})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	_, err = d.Pool.Exec(ctx, `
		INSERT INTO audit_runs (id, project_id, organization_id, status, standard_code, standard_version, rules_sha256, evidence_manifest, created_by)
		VALUES ($1, $2, $3, 'COMPLETED', 'IRRS', '0.1.0', 'testsha', '[]'::jsonb, $4)
	`, runID, project.ID, actor.OrganizationID, actor.UserID)
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	_, err = d.Pool.Exec(ctx, `
		INSERT INTO findings (id, audit_run_id, organization_id, rule_id, assessment_status, severity, confidence, review_status, effective_status)
		VALUES ($1, $2, $3, 'IRRS-D2-001', 'FAIL', 'S1', 0.9, 'CONFIRMED', 'FAIL')
	`, findingID, runID, actor.OrganizationID)
	if err != nil {
		t.Fatalf("seed finding: %v", err)
	}
	return findingID
}

func TestTaskVersionMismatchConflicts(t *testing.T) {
	d := testDB(t)
	actor := testutil.SeedOrgUser(t, d.Pool, "task-v-"+uuid.NewString()[:8]+"@org.test")
	findingID := seedFindingForTask(t, d, actor)
	ctx := context.Background()

	if _, err := d.TransitionFixTask(ctx, actor, findingID, "ACKNOWLEDGED", "", "t", 1); err != nil {
		t.Fatalf("first transition: %v", err)
	}
	// Stale version (task is now v2) must conflict, not silently overwrite.
	if _, err := d.TransitionFixTask(ctx, actor, findingID, "FIXING", "", "t", 1); !errors.Is(err, ErrJobConflict) {
		t.Fatalf("want ErrJobConflict for stale version, got %v", err)
	}
	// Missing version entirely is rejected up front.
	if _, err := d.TransitionFixTask(ctx, actor, findingID, "FIXING", "", "t", 0); err == nil {
		t.Fatal("want error for missing version, got nil")
	}
}

func TestResolveRequiresRetestPass(t *testing.T) {
	d := testDB(t)
	actor := testutil.SeedOrgUser(t, d.Pool, "task-r-"+uuid.NewString()[:8]+"@org.test")
	findingID := seedFindingForTask(t, d, actor)
	ctx := context.Background()

	steps := []struct{ to, reason string }{
		{"ACKNOWLEDGED", ""}, {"FIXING", ""}, {"READY_FOR_RETEST", ""},
	}
	version := 1
	for _, s := range steps {
		task, err := d.TransitionFixTask(ctx, actor, findingID, s.to, s.reason, "t", version)
		if err != nil {
			t.Fatalf("transition to %s: %v", s.to, err)
		}
		version = task.Version
	}
	// No retest run exists yet → RESOLVED must be refused.
	if _, err := d.TransitionFixTask(ctx, actor, findingID, "RESOLVED", "", "t", version); !errors.Is(err, ErrResolveWithoutRetest) {
		t.Fatalf("want ErrResolveWithoutRetest, got %v", err)
	}
}

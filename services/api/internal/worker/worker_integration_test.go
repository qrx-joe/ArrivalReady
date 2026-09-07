package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/testutil"
)

// B07 integration suite (execution plan verify): claim concurrency, crash
// recovery via lease expiry, cancellation of late results, retry exhaustion,
// and atomic persistence — against REAL PostgreSQL.
// TEST_DATABASE_URL must point at a throwaway database.
func env(t *testing.T) (*store.DB, auth.Actor, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set: B07 integration suite requires throwaway PostgreSQL")
	}
	pool := testutil.ConnectPG(t, dsn)
	testutil.MigrateTestDB(t, dsn)
	db := &store.DB{Pool: pool}

	// The jobs queue is system-global (workers have no org scope), so stale
	// jobs from earlier runs of the suite would be claimed by whichever test
	// gets there first. Every test starts from a clean queue; the throwaway
	// test DB makes this delete safe. Reviews and fix tasks reference
	// findings (0006), so they go first.
	for _, stmt := range []string{
		`DELETE FROM fix_task_events`, `DELETE FROM fix_tasks`, `DELETE FROM reviews`,
		`DELETE FROM finding_evidence`, `DELETE FROM findings`,
		`DELETE FROM jobs`, `DELETE FROM audit_runs`,
	} {
		if _, err := pool.Exec(context.Background(), stmt); err != nil {
			t.Fatalf("reset queue (%s): %v", stmt, err)
		}
	}

	actor := testutil.SeedOrgUser(t, pool, fmt.Sprintf("b07-%d@org.test", time.Now().UnixNano()))
	rulesPath, err := ResolveStandardsPath("")
	if err != nil {
		t.Fatalf("resolve rules: %v", err)
	}
	return db, actor, rulesPath
}

func newWorker(t *testing.T, db *store.DB, ai AIClient, rulesPath string, lease time.Duration) *Worker {
	t.Helper()
	return &Worker{
		DB: db, AI: ai, RulesPath: rulesPath, Lease: lease, Interval: time.Hour,
		BuildPayload: BuildAuditPayload(db, nil, rulesPath),
	}
}

func seedProject(t *testing.T, db *store.DB, actor auth.Actor) uuid.UUID {
	t.Helper()
	p, err := db.CreateProject(context.Background(), actor, store.CreateProjectInput{
		Name: "B07 测试项目", EntityType: "restaurant", TargetLocale: "en-US",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func seedReadyEvidence(t *testing.T, db *store.DB, actor auth.Actor, projectID uuid.UUID, n int) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		e, err := db.CreateEvidence(ctx, actor, store.CreateEvidenceInput{
			ProjectID: projectID, Type: "image", ContentType: "image/jpeg",
			SizeBytes: 100, JourneyStage: "act", ObjectKey: fmt.Sprintf("test/%s/%d-%d.jpg", projectID, time.Now().UnixNano(), i),
		})
		if err != nil {
			t.Fatal(err)
		}
		sha := fmt.Sprintf("sha-%s", e.ID)
		if _, err := db.MarkEvidenceReady(ctx, actor, e.ID, sha, "fp-"+sha); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
	}
	return ids
}

// fakeAI stands in for the Python service. Like the real endpoint, it echoes
// the request's attempt_token back into the envelope metadata, so tests never
// need to pre-compute tokens.
type fakeAI struct {
	mu      sync.Mutex
	handler func(payload map[string]any, call int) (status int, body map[string]any)
	Calls   int
}

func (f *fakeAI) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.mu.Lock()
		f.Calls++
		call := f.Calls
		f.mu.Unlock()
		status, body := f.handler(payload, call)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func envelope(payload map[string]any, outcome string, extra map[string]any) map[string]any {
	meta := map[string]any{
		"job_id": payload["job_id"], "run_id": payload["run_id"], "attempt_token": unquote(payload["attempt_token"]),
		"provider": "fake", "model": "fake-1", "model_version": nil,
		"prompt_version": "assessment/v1", "schema_version": "assessment-1.0",
		"input_hash":   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"requested_at": "2026-09-07T10:00:00Z",
	}
	body := map[string]any{"outcome": outcome, "metadata": meta}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func unquote(v any) string {
	s, _ := v.(string)
	return s
}

func successBody(payload map[string]any, ruleID string, evidenceID string) map[string]any {
	refs := []any{}
	if evidenceID != "" {
		refs = append(refs, map[string]any{
			"evidence_id": evidenceID,
			"locator":     map[string]any{"type": "bbox", "bbox": map[string]any{"x": 0.1, "y": 0.1, "w": 0.3, "h": 0.3}},
		})
	}
	return envelope(payload, "success", map[string]any{
		"assessments": []any{map[string]any{
			"rule_id": ruleID, "status": "FAIL", "severity": "S1",
			"evidence_refs": refs,
			"observation":   "落地页仅中文", "reason": "目标访客无法独立下单",
			"confidence": 0.91, "needs_human_review": false,
		}},
	})
}

func failureBody(payload map[string]any, retryable bool) map[string]any {
	return envelope(payload, "failure", map[string]any{
		"error": map[string]any{"code": "provider_auth", "retryable": retryable, "message": "bad key"},
	})
}

func firstManifestEvidence(db *store.DB, runID uuid.UUID) uuid.UUID {
	run, err := db.GetRunInternal(context.Background(), runID)
	if err != nil {
		return uuid.Nil
	}
	var manifest []store.ManifestEntry
	_ = json.Unmarshal(run.EvidenceManifestJSON, &manifest)
	if len(manifest) == 0 {
		return uuid.Nil
	}
	return manifest[0].EvidenceID
}

func TestHappyPathPersistsFindingsAtomically(t *testing.T) {
	db, actor, rulesPath := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	evIDs := seedReadyEvidence(t, db, actor, projectID, 1)

	// The fake echoes the claimed token and cites the run's real evidence.
	srv := (&fakeAI{handler: func(payload map[string]any, _ int) (int, map[string]any) {
		runID, _ := uuid.Parse(fmt.Sprint(payload["run_id"]))
		return http.StatusOK, successBody(payload, "IRRS-D4-001", firstManifestEvidence(db, runID).String())
	}}).server()
	defer srv.Close()

	run, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: evIDs, StandardVersion: "0.1.0", RulesSHA256: "test-hash",
	})
	if err != nil {
		t.Fatalf("create audit: %v", err)
	}

	w := newWorker(t, db, &HTTPAIClient{BaseURL: srv.URL}, rulesPath, time.Minute)
	if err := w.ProcessOne(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	final, err := db.GetRun(ctx, actor, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "REVIEW_REQUIRED" {
		t.Fatalf("run status = %s, want REVIEW_REQUIRED", final.Status)
	}
	findings, err := db.ListFindings(ctx, actor, run.ID)
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings = %d, err=%v, want 1", len(findings), err)
	}
}

func TestConcurrentClaimNeverDoubles(t *testing.T) {
	db, actor, rulesPath := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	evIDs := seedReadyEvidence(t, db, actor, projectID, 1)
	run, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: evIDs, StandardVersion: "0.1.0", RulesSHA256: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	// The jobs table is global across orgs (workers are system-level), so
	// leftover claimable jobs from other tests may exist and be claimed by
	// other goroutines — that is fine. What must NEVER happen is OUR job
	// being claimed more than once in the same lease window.
	var ourJobID uuid.UUID
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM jobs WHERE run_id = $1`, run.ID).Scan(&ourJobID); err != nil {
		t.Fatal(err)
	}

	w := newWorker(t, db, &HTTPAIClient{BaseURL: "http://127.0.0.1:1"}, rulesPath, time.Minute)

	const workers = 4
	results := make(chan *store.ClaimedJob, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := w.DB.ClaimNextJob(ctx, time.Minute)
			if err != nil {
				results <- nil
				return
			}
			results <- c
		}()
	}
	wg.Wait()
	close(results)

	claimed := 0
	ownClaims := 0
	for c := range results {
		if c == nil {
			continue
		}
		claimed++
		if c.ID == ourJobID {
			ownClaims++
		}
	}
	if ownClaims > 1 {
		t.Fatalf("our job %s was claimed %d times concurrently", ourJobID, ownClaims)
	}
	var attempts int
	if err := db.Pool.QueryRow(ctx, `SELECT attempts FROM jobs WHERE id = $1`, ourJobID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("attempts on our job = %d, want exactly 1", attempts)
	}
	if claimed > workers {
		t.Fatalf("impossible claim count %d", claimed)
	}
}

func TestLeaseExpiryDiscardsLateResult(t *testing.T) {
	db, actor, rulesPath := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	evIDs := seedReadyEvidence(t, db, actor, projectID, 1)
	if _, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: evIDs, StandardVersion: "0.1.0", RulesSHA256: "h",
	}); err != nil {
		t.Fatal(err)
	}

	w := newWorker(t, db, &HTTPAIClient{BaseURL: "http://127.0.0.1:1"}, rulesPath, 50*time.Millisecond)
	claimed, err := w.DB.ClaimNextJob(ctx, 50*time.Millisecond)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v", err)
	}
	time.Sleep(80 * time.Millisecond) // lease lapses

	// Late success from the expired attempt must be discarded.
	applied, err := db.CompleteJobAttempt(ctx, claimed.ID, *claimed.AttemptToken, []store.PersistentFinding{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expired-lease result must not be applied")
	}

	// Crash recovery: the job is claimable again with a FRESH token.
	reclaimed, err := w.DB.ClaimNextJob(ctx, time.Minute)
	if err != nil || reclaimed == nil {
		t.Fatalf("reclaim after expiry: %v", err)
	}
	if reclaimed.ID != claimed.ID {
		t.Fatalf("reclaimed %s, want original %s", reclaimed.ID, claimed.ID)
	}
	if reclaimed.Attempts != 2 {
		t.Fatalf("attempts after reclaim = %d, want 2", reclaimed.Attempts)
	}
	if reclaimed.AttemptToken == nil || *reclaimed.AttemptToken == *claimed.AttemptToken {
		t.Fatal("reclaim must issue a new attempt token")
	}
}

func TestRetryExhaustionFailsRun(t *testing.T) {
	db, actor, rulesPath := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	evIDs := seedReadyEvidence(t, db, actor, projectID, 1)
	run, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: evIDs, StandardVersion: "0.1.0", RulesSHA256: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = run
	// Transport always fails (dead port) → ReleaseJobAttempt on each attempt.
	w := newWorker(t, db, &HTTPAIClient{BaseURL: "http://127.0.0.1:1"}, rulesPath, time.Second)
	for i := 0; i < 3; i++ {
		if err := w.ProcessOne(ctx); err != nil {
			t.Fatalf("process %d: %v", i, err)
		}
	}

	final, err := db.GetRun(ctx, actor, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "FAILED" {
		t.Fatalf("run status = %s, want FAILED after attempts exhausted", final.Status)
	}
	if !strings.Contains(final.StatusReason, "attempts exhausted") {
		t.Fatalf("status_reason = %q", final.StatusReason)
	}
}

func TestDefinitiveFailureFailsRunImmediately(t *testing.T) {
	db, actor, rulesPath := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	evIDs := seedReadyEvidence(t, db, actor, projectID, 1)
	run, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: evIDs, StandardVersion: "0.1.0", RulesSHA256: "h",
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := (&fakeAI{handler: func(payload map[string]any, _ int) (int, map[string]any) {
		return http.StatusOK, failureBody(payload, false) // non-retryable
	}}).server()
	defer srv.Close()

	w := newWorker(t, db, &HTTPAIClient{BaseURL: srv.URL}, rulesPath, time.Minute)
	if err := w.ProcessOne(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	final, err := db.GetRun(ctx, actor, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "FAILED" || !strings.Contains(final.StatusReason, "provider_auth") {
		t.Fatalf("run = %s/%q, want FAILED/provider_auth", final.Status, final.StatusReason)
	}
}

func TestCancelDiscardsLateSuccess(t *testing.T) {
	db, actor, _ := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	evIDs := seedReadyEvidence(t, db, actor, projectID, 1)
	run, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: evIDs, StandardVersion: "0.1.0", RulesSHA256: "h",
	})
	if err != nil {
		t.Fatal(err)
	}

	var claimed *store.ClaimedJob
	// Claim holds the job; we cancel BEFORE the AI answers.
	claimed, err = db.ClaimNextJob(ctx, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := db.CancelRun(ctx, actor, run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// The late success must not resurrect the cancelled run.
	applied, err := db.CompleteJobAttempt(ctx, claimed.ID, *claimed.AttemptToken, []store.PersistentFinding{
		{RuleID: "IRRS-D4-001", Status: "FAIL", Severity: "S1", Confidence: 0.5},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("late success after cancel must be discarded")
	}
	final, err := db.GetRun(ctx, actor, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "CANCELLED" {
		t.Fatalf("run status = %s, want CANCELLED", final.Status)
	}
	findings, _ := db.ListFindings(ctx, actor, run.ID)
	if len(findings) != 0 {
		t.Fatalf("cancelled run must have no findings, got %d", len(findings))
	}
}

func TestGroundDropsInvalidAssessments(t *testing.T) {
	rules := map[string]bool{"IRRS-D4-001": true}
	manifest := []store.ManifestEntry{{EvidenceID: uuid.Must(uuid.NewV7())}}
	good := manifest[0]

	out := &providerResponse{
		Outcome: "success",
		Assessments: []providerAssessment{
			{RuleID: "IRRS-D4-001", Status: "FAIL", Severity: "S1", Confidence: 0.9,
				EvidenceRefs: []providerRef{{EvidenceID: good.EvidenceID.String(), Locator: json.RawMessage(`{"type":"full"}`)}}},
			{RuleID: "IRRS-XX-999", Status: "FAIL", Severity: "S1", Confidence: 0.9},    // unknown rule
			{RuleID: "IRRS-D4-001", Status: "MAYBE", Severity: "S1", Confidence: 0.9},   // bad enum
			{RuleID: "IRRS-D4-001", Status: "FAIL", Severity: "S1", Confidence: 1.2},    // bad confidence
			{RuleID: "IRRS-D4-001", Status: "FAIL", Severity: "S1", Confidence: 0.9},    // no evidence
			{RuleID: "IRRS-D4-001", Status: "UNKNOWN", Severity: "S1", Confidence: 0.4}, // unknown w/o refs is fine
		},
	}
	findings, err := Ground(out, manifest, rules)
	if err != nil {
		t.Fatalf("ground: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("grounded findings = %d, want 2 (valid FAIL + honest UNKNOWN)", len(findings))
	}
	if len(findings[0].EvidenceRefs) != 1 {
		t.Fatalf("valid FAIL must keep its evidence link")
	}
	if len(findings[1].EvidenceRefs) != 0 {
		t.Fatalf("UNKNOWN must not fabricate evidence links")
	}

	// ALL assessments invalid → hard error, worker fails the job definitively.
	out2 := &providerResponse{Outcome: "success", Assessments: []providerAssessment{
		{RuleID: "IRRS-XX-999", Status: "FAIL", Severity: "S1", Confidence: 0.9},
	}}
	if _, err := Ground(out2, manifest, rules); err == nil {
		t.Fatal("all-invalid assessments must be a hard grounding error")
	}
}

func TestPayloadMissingEvidenceIsRejected(t *testing.T) {
	db, actor, _ := env(t)
	ctx := context.Background()
	projectID := seedProject(t, db, actor)
	quarantined, err := db.CreateEvidence(ctx, actor, store.CreateEvidenceInput{
		ProjectID: projectID, Type: "image", ContentType: "image/jpeg",
		SizeBytes: 10, JourneyStage: "act", ObjectKey: fmt.Sprintf("q/%d.jpg", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Quarantined evidence can never enter a run's frozen manifest (B06/B07).
	if _, err := db.CreateAuditWithJob(ctx, actor, projectID, store.CreateAuditInput{
		EvidenceIDs: []uuid.UUID{quarantined.ID}, StandardVersion: "0.1.0", RulesSHA256: "h",
	}); err == nil {
		t.Fatal("quarantined evidence must be rejected at audit creation")
	}
}

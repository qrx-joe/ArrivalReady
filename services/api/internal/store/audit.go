package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

// Audit job lifecycle persistence (execution plan B07, R-11).
//
// The three protocols here are the concurrency core:
//   - ClaimNextJob: FOR UPDATE SKIP LOCKED so concurrent workers never claim
//     the same job; the claim transaction also flips the run QUEUED→ANALYZING,
//     which is where a user cancel is detected before any work starts.
//   - ReleaseJobAttempt: transient failure → back to QUEUED (retried by the
//     next claim); attempts already incremented, so exhaustion is visible.
//   - CompleteJobAttempt: guarded by attempt_token AND unexpired lease AND
//     non-terminal run — a stale/cancelled attempt matches no rows and is
//     discarded WITHOUT persisting anything. Findings, evidence links and the
//     run transition land in ONE transaction (§5.2 atomicity).

var ErrJobConflict = errors.New("job attempt no longer valid")

const defaultLease = 3 * time.Minute

type AuditRun struct {
	ID                   uuid.UUID       `json:"id"`
	ProjectID            uuid.UUID       `json:"project_id"`
	OrganizationID       uuid.UUID       `json:"organization_id"`
	ParentRunID          *uuid.UUID      `json:"parent_run_id"`
	Status               string          `json:"status"`
	StatusReason         string          `json:"status_reason"`
	StandardCode         string          `json:"standard_code"`
	StandardVersion      string          `json:"standard_version"`
	RulesSHA256          string          `json:"rules_sha256"`
	EvidenceManifestJSON json.RawMessage `json:"evidence_manifest"`
	// ScoringJSON is the report frozen by finalize (nil before finalize);
	// the run is read-only afterwards, so it can be returned as-is.
	ScoringJSON json.RawMessage `json:"scoring"`
	CreatedBy   uuid.UUID       `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at"`
}

type Job struct {
	ID           uuid.UUID  `json:"id"`
	RunID        uuid.UUID  `json:"run_id"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	MaxAttempts  int        `json:"max_attempts"`
	AttemptToken *uuid.UUID `json:"attempt_token"`
	LeaseUntil   *time.Time `json:"lease_until"`
}

type ManifestEntry struct {
	EvidenceID uuid.UUID `json:"evidence_id"`
	SHA256     string    `json:"sha256"`
}

type CreateAuditInput struct {
	EvidenceIDs     []uuid.UUID
	StandardVersion string
	RulesSHA256     string
}

// CreateAuditWithJob freezes the evidence input snapshot and creates run + job
// in one transaction (B07 step ① ②).
func (d *DB) CreateAuditWithJob(ctx context.Context, actor auth.Actor, projectID uuid.UUID, in CreateAuditInput) (AuditRun, error) {
	if _, err := d.GetProject(ctx, actor, projectID); err != nil {
		return AuditRun{}, err
	}

	manifest := make([]ManifestEntry, 0, len(in.EvidenceIDs))
	for _, id := range in.EvidenceIDs {
		e, err := d.GetEvidence(ctx, actor, id)
		if err != nil {
			return AuditRun{}, err
		}
		if e.ProcessingStatus != "READY" {
			return AuditRun{}, errors.New("evidence " + id.String() + " is " + e.ProcessingStatus + "; only READY evidence can be audited")
		}
		manifest = append(manifest, ManifestEntry{EvidenceID: e.ID, SHA256: e.SHA256})
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return AuditRun{}, err
	}

	runID, err := uuid.NewV7()
	if err != nil {
		return AuditRun{}, err
	}
	jobID, err := uuid.NewV7()
	if err != nil {
		return AuditRun{}, err
	}

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return AuditRun{}, err
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_runs (id, project_id, organization_id, status, standard_version, rules_sha256, evidence_manifest, created_by)
		VALUES ($1,$2,$3,'QUEUED',$4,$5,$6,$7)
	`, runID, projectID, actor.OrganizationID, in.StandardVersion, in.RulesSHA256, manifestJSON, actor.UserID); err != nil {
		return AuditRun{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO jobs (id, run_id, organization_id) VALUES ($1,$2,$3)
	`, jobID, runID, actor.OrganizationID); err != nil {
		return AuditRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AuditRun{}, err
	}
	return d.GetRun(ctx, actor, runID)
}

const runColumns = `id, project_id, organization_id, parent_run_id, status,
	COALESCE(status_reason,''), standard_code, standard_version, rules_sha256,
	evidence_manifest, scoring, created_by, created_at, completed_at`

func scanRun(row pgx.Row) (AuditRun, error) {
	var r AuditRun
	err := row.Scan(&r.ID, &r.ProjectID, &r.OrganizationID, &r.ParentRunID, &r.Status,
		&r.StatusReason, &r.StandardCode, &r.StandardVersion, &r.RulesSHA256,
		&r.EvidenceManifestJSON, &r.ScoringJSON, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt)
	return r, err
}

func (d *DB) GetRun(ctx context.Context, actor auth.Actor, runID uuid.UUID) (AuditRun, error) {
	r, err := scanRun(d.Pool.QueryRow(ctx,
		`SELECT `+runColumns+` FROM audit_runs WHERE id = $1 AND organization_id = $2`,
		runID, actor.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AuditRun{}, ErrNotFound
	}
	return r, err
}

func (d *DB) ListRuns(ctx context.Context, actor auth.Actor, projectID uuid.UUID) ([]AuditRun, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT `+runColumns+` FROM audit_runs
		WHERE project_id = $1 AND organization_id = $2
		ORDER BY created_at DESC
	`, projectID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditRun{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRunInternal loads a run WITHOUT the organization scope, for server-side
// subsystems that already hold full context (worker payload assembly, tests).
// Request-facing code must use the actor-scoped GetRun.
func (d *DB) GetRunInternal(ctx context.Context, runID uuid.UUID) (AuditRun, error) {
	r, err := scanRun(d.Pool.QueryRow(ctx,
		`SELECT `+runColumns+` FROM audit_runs WHERE id = $1`, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AuditRun{}, ErrNotFound
	}
	return r, err
}

// findingTaskJSON is the fix-task block for finding rows. Tasks are created
// lazily on first transition (EnsureFixTask), so before that the reader gets
// the effective OPEN/v1 view with a null id — the shape the UI and contract
// both expect without forcing a write on GET.
const findingTaskJSON = `'task', COALESCE((
				SELECT json_build_object(
					'id', t.id, 'finding_id', t.finding_id,
					'workflow_status', t.workflow_status, 'version', t.version,
					'created_at', t.created_at)
				FROM fix_tasks t WHERE t.finding_id = f.id
			), json_build_object(
				'id', NULL, 'finding_id', f.id,
				'workflow_status', 'OPEN', 'version', 1,
				'created_at', f.created_at))`

// GetFindingJSON returns one finding (org-scoped) shaped like the list rows,
// with its evidence refs and locators inline for the Evidence Viewer.
func (d *DB) GetFindingJSON(ctx context.Context, actor auth.Actor, findingID uuid.UUID) (json.RawMessage, error) {
	var raw json.RawMessage
	err := d.Pool.QueryRow(ctx, `
		SELECT json_build_object(
			'id', f.id, 'audit_run_id', f.audit_run_id,
			'rule_id', f.rule_id, 'assessment_status', f.assessment_status,
			'severity', f.severity, 'title', f.title, 'observation', f.observation,
			'reason', f.reason, 'recommended_fix', f.recommended_fix,
			'confidence', f.confidence, 'review_status', f.review_status,
			'created_at', f.created_at, 'original_candidate', f.original_candidate,
			`+findingTaskJSON+`,
			'evidence_refs', COALESCE((
				SELECT json_agg(json_build_object(
					'evidence_id', fe.evidence_id,
					'locator', fe.locator))
				FROM finding_evidence fe WHERE fe.finding_id = f.id
			), '[]'::json))
		FROM findings f
		WHERE f.id = $1 AND f.organization_id = $2
	`, findingID, actor.OrganizationID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return raw, err
}

// ListFindings returns candidate findings of a run as JSON rows (org-scoped).
func (d *DB) ListFindings(ctx context.Context, actor auth.Actor, runID uuid.UUID) ([]json.RawMessage, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT json_build_object(
			'id', f.id, 'audit_run_id', f.audit_run_id,
			'rule_id', f.rule_id, 'assessment_status', f.assessment_status,
			'severity', f.severity, 'title', f.title, 'observation', f.observation,
			'reason', f.reason, 'recommended_fix', f.recommended_fix,
			'confidence', f.confidence, 'review_status', f.review_status,
			'created_at', f.created_at, 'original_candidate', f.original_candidate,
			`+findingTaskJSON+`,
			'evidence_refs', COALESCE((
				SELECT json_agg(json_build_object(
					'evidence_id', fe.evidence_id,
					'locator', fe.locator))
				FROM finding_evidence fe WHERE fe.finding_id = f.id
			), '[]'::json))
		FROM findings f
		WHERE f.audit_run_id = $1 AND f.organization_id = $2
		ORDER BY f.created_at
	`, runID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

// CancelRun flips a non-terminal run to CANCELLED. A QUEUED job is cancelled
// immediately; a RUNNING job is left for the worker, whose completion guard
// will detect the cancelled run and discard (or cancel) the attempt.
func (d *DB) CancelRun(ctx context.Context, actor auth.Actor, runID uuid.UUID) (AuditRun, error) {
	tag, err := d.Pool.Exec(ctx, `
		UPDATE audit_runs SET status = 'CANCELLED', status_reason = 'cancelled by user'
		WHERE id = $1 AND organization_id = $2
		  AND status IN ('QUEUED','INGESTING','ANALYZING','REVIEW_REQUIRED')
	`, runID, actor.OrganizationID)
	if err != nil {
		return AuditRun{}, err
	}
	if tag.RowsAffected() == 0 {
		r, gerr := d.GetRun(ctx, actor, runID)
		if gerr != nil {
			return AuditRun{}, gerr
		}
		return r, ErrJobConflict // terminal runs cannot be cancelled
	}
	if _, err := d.Pool.Exec(ctx, `
		UPDATE jobs SET status = 'CANCELLED', updated_at = now()
		WHERE run_id = $1 AND status = 'QUEUED'
	`, runID); err != nil {
		return AuditRun{}, err
	}
	return d.GetRun(ctx, actor, runID)
}

// ClaimedJob is the exclusive work grant handed to one worker.
type ClaimedJob struct {
	Job
	StandardVersion string
	RulesSHA256     string
	Manifest        []ManifestEntry
}

// ClaimNextJob leases one runnable job (QUEUED, or RUNNING with an EXPIRED
// lease — the crash-recovery path). Concurrent workers block-free skip each
// other's rows, so a job is claimed at most once per lease window. The same
// transaction moves the run QUEUED→ANALYZING; a cancelled run disqualifies
// its job here, before any provider spend.
func (d *DB) ClaimNextJob(ctx context.Context, lease time.Duration) (*ClaimedJob, error) {
	// Callers that forget the lease would otherwise mint instantly-expired
	// leases whose results are ALWAYS discarded by the completion guard —
	// silently losing every successful assessment (B07 E2E 实测缺陷).
	if lease <= 0 {
		lease = defaultLease
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	var jobID, runID uuid.UUID
	var attempts, maxAttempts int
	err = tx.QueryRow(ctx, `
		SELECT j.id, j.run_id, j.attempts, j.max_attempts
		FROM jobs j
		JOIN audit_runs r ON r.id = j.run_id
		WHERE (j.status = 'QUEUED' OR (j.status = 'RUNNING' AND j.lease_until < now()))
		  AND j.attempts < j.max_attempts
		  AND r.status IN ('QUEUED','ANALYZING')
		ORDER BY j.created_at
		FOR UPDATE OF j SKIP LOCKED
		LIMIT 1
	`).Scan(&jobID, &runID, &attempts, &maxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // empty queue is not an error
	}
	if err != nil {
		return nil, err
	}

	token, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	until := time.Now().Add(lease)
	if _, err := tx.Exec(ctx, `
		UPDATE jobs SET status = 'RUNNING', attempts = attempts + 1,
		       attempt_token = $2, lease_until = $3, updated_at = now()
		WHERE id = $1
	`, jobID, token, until); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE audit_runs SET status = 'ANALYZING'
		WHERE id = $1 AND status IN ('QUEUED','ANALYZING')
	`, runID); err != nil {
		return nil, err
	}

	var stdVersion, rulesHash string
	var manifestJSON []byte
	if err := tx.QueryRow(ctx, `
		SELECT standard_version, rules_sha256, evidence_manifest
		FROM audit_runs WHERE id = $1
	`, runID).Scan(&stdVersion, &rulesHash, &manifestJSON); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	manifest := []ManifestEntry{}
	if len(manifestJSON) > 0 {
		if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
			return nil, err
		}
	}
	return &ClaimedJob{
		Job:             Job{ID: jobID, RunID: runID, Status: "RUNNING", Attempts: attempts + 1, MaxAttempts: maxAttempts, AttemptToken: &token, LeaseUntil: &until},
		StandardVersion: stdVersion,
		RulesSHA256:     rulesHash,
		Manifest:        manifest,
	}, nil
}

// ReleaseJobAttempt handles a RETRYABLE failure: the attempt goes back to
// QUEUED for another claim. If attempts are exhausted the job fails for good
// and takes its run with it — a job must never stall in QUEUED forever.
func (d *DB) ReleaseJobAttempt(ctx context.Context, jobID, token uuid.UUID, reason string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	var attempts, maxAttempts int
	var runID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE jobs SET status = 'QUEUED', attempt_token = NULL, lease_until = NULL,
		       last_error = $3, updated_at = now()
		WHERE id = $1 AND attempt_token = $2 AND status = 'RUNNING'
		RETURNING attempts, max_attempts, run_id
	`, jobID, token, reason).Scan(&attempts, &maxAttempts, &runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // stale attempt: discard
	}
	if err != nil {
		return err
	}

	if attempts >= maxAttempts {
		if _, err := tx.Exec(ctx, `
			UPDATE jobs SET status = 'FAILED', updated_at = now() WHERE id = $1
		`, jobID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE audit_runs SET status = 'FAILED', status_reason = $2, completed_at = now()
			WHERE id = $1 AND status NOT IN ('COMPLETED','FAILED','CANCELLED')
		`, runID, "attempts exhausted: "+reason); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PersistentFinding is one grounded candidate finding ready for persistence.
type PersistentFinding struct {
	RuleID         string
	Status         string
	Severity       string
	Title          string
	Observation    string
	Reason         string
	RecommendedFix string
	Confidence     float64
	Original       json.RawMessage // full AI candidate, replayable (R-05)
	EvidenceRefs   []EvidenceLink
}

type EvidenceLink struct {
	EvidenceID uuid.UUID
	Locator    json.RawMessage
}

// CompleteJobAttempt is the SINGLE persistence gate (B07 step ⑤).
//
// Guard order inside one transaction: token match (stale attempt?) →
// unexpired lease (late attempt?) → run still non-terminal (cancelled?).
// Any guard failing aborts everything: applied=false, nothing written.
// On success: job SUCCEEDED, findings + evidence links inserted, run →
// REVIEW_REQUIRED — atomically, so a run is never REVIEW_REQUIRED without
// all of its findings.
func (d *DB) CompleteJobAttempt(ctx context.Context, jobID, token uuid.UUID, findings []PersistentFinding, reason string) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())

	var runID, orgID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE jobs SET status = 'SUCCEEDED', attempt_token = NULL, lease_until = NULL, updated_at = now()
		WHERE id = $1 AND attempt_token = $2 AND status = 'RUNNING' AND lease_until >= now()
		RETURNING run_id, organization_id
	`, jobID, token).Scan(&runID, &orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // stale, late or expired attempt — discard silently
	}
	if err != nil {
		return false, err
	}

	// Run guard FIRST: if the run was cancelled while we worked, nothing may
	// be written at all. Inserting findings before this check would commit
	// them via the final COMMIT even when we report "discarded".
	tag, err := tx.Exec(ctx, `
		UPDATE audit_runs SET status = 'REVIEW_REQUIRED', completed_at = now()
		WHERE id = $1 AND status NOT IN ('COMPLETED','FAILED','CANCELLED')
	`, runID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE jobs SET status = 'CANCELLED', attempt_token = NULL, lease_until = NULL, updated_at = now()
			WHERE id = $1
		`, jobID); err != nil {
			return false, err
		}
		return false, tx.Commit(ctx)
	}

	for _, f := range findings {
		fid, err := uuid.NewV7()
		if err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO findings (id, audit_run_id, organization_id, rule_id, assessment_status,
			                      severity, title, observation, reason, recommended_fix,
			                      confidence, original_candidate)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (audit_run_id, rule_id) DO NOTHING
		`, fid, runID, orgID, f.RuleID, f.Status, f.Severity, f.Title, f.Observation,
			f.Reason, f.RecommendedFix, f.Confidence, f.Original); err != nil {
			return false, err
		}
		for _, ref := range f.EvidenceRefs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO finding_evidence (finding_id, evidence_id, locator)
				VALUES ($1,$2,$3)
				ON CONFLICT DO NOTHING
			`, fid, ref.EvidenceID, ref.Locator); err != nil {
				return false, err
			}
		}
	}

	return true, tx.Commit(ctx)
}

// FailJobAttempt is the definitive-failure path (non-retryable provider
// errors, grounding catastrophes): job FAILED, run FAILED with reason. Same
// token/lease guards as CompleteJobAttempt.
func (d *DB) FailJobAttempt(ctx context.Context, jobID, token uuid.UUID, reason string) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())

	var runID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE jobs SET status = 'FAILED', attempt_token = NULL, lease_until = NULL,
		       last_error = $3, updated_at = now()
		WHERE id = $1 AND attempt_token = $2 AND status = 'RUNNING' AND lease_until >= now()
		RETURNING run_id
	`, jobID, token, reason).Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE audit_runs SET status = 'FAILED', status_reason = $2, completed_at = now()
		WHERE id = $1 AND status NOT IN ('COMPLETED','FAILED','CANCELLED')
	`, runID, reason)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE jobs SET status = 'CANCELLED', attempt_token = NULL, lease_until = NULL, updated_at = now()
			WHERE id = $1
		`, jobID); err != nil {
			return false, err
		}
		return false, tx.Commit(ctx)
	}
	return true, tx.Commit(ctx)
}

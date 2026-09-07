package store

// Review + finalize persistence (B10, ADR-0002 / D-018).

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

type ReviewInput struct {
	Decision  string // confirm/reject/edit/na
	Note      string
	NewStatus string // edit 时的新结论
	NAReason  string
}

// SubmitReview records the human decision (append-only reviews row) and moves
// the finding UNREVIEWED → its terminal review state in ONE transaction. A
// finding not in UNREVIEWED conflicts — decisions are never overwritten
// (状态迁移表 §3). Reject keeps effective=UNKNOWN: 驳回且无替代判断不产生 PASS
// (ADR-0002 §2.5); NA requires a reason and shrinks the applicable set.
func (d *DB) SubmitReview(ctx context.Context, actor auth.Actor, findingID uuid.UUID, in ReviewInput, requestID string) (json.RawMessage, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var reviewStatus, effective, assessment string
	err = tx.QueryRow(ctx, `
		SELECT review_status, COALESCE(effective_status, ''), assessment_status
		FROM findings WHERE id = $1 AND organization_id = $2
		FOR UPDATE
	`, findingID, actor.OrganizationID).Scan(&reviewStatus, &effective, &assessment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if reviewStatus != "UNREVIEWED" {
		return nil, ErrJobConflict
	}

	switch in.Decision {
	case "confirm":
		effective = assessment
		reviewStatus = "CONFIRMED"
	case "reject":
		effective = "UNKNOWN"
		reviewStatus = "REJECTED"
	case "edit":
		if in.NewStatus == "" {
			return nil, errors.New("edit requires a new status")
		}
		effective = in.NewStatus
		reviewStatus = "EDITED"
	case "na":
		if in.NAReason == "" {
			return nil, errors.New("na requires a reason")
		}
		effective = "NA"
		reviewStatus = "NA"
	default:
		return nil, errors.New("unknown decision " + in.Decision)
	}

	reviewID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO reviews (id, finding_id, reviewer_id, decision, note, na_reason)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, reviewID, findingID, actor.UserID, in.Decision, nullIfEmpty(in.Note), nullIfEmpty(in.NAReason)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE findings SET review_status = $2, effective_status = $3, na_reason = $4
		WHERE id = $1
	`, findingID, reviewStatus, effective, nullIfEmpty(in.NAReason)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log (id, actor_id, organization_id, action, resource_type, resource_id, request_id)
		SELECT $1, $2, organization_id, 'finding.reviewed', 'finding', id, $4
		FROM findings WHERE id = $3
	`, mustUUID(), actor.UserID, findingID, requestID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return d.GetFindingJSON(ctx, actor, findingID)
}

// ScoringRow is one finding's post-review scoring inputs.
type ScoringRow struct {
	RuleID           string
	Severity         string
	Effective        string
	ReviewStatus     string
	AssessmentStatus string
}

func (d *DB) ScoringRows(ctx context.Context, actor auth.Actor, runID uuid.UUID) ([]ScoringRow, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT rule_id, severity, COALESCE(effective_status, assessment_status),
		       review_status, assessment_status
		FROM findings
		WHERE audit_run_id = $1 AND organization_id = $2
		ORDER BY created_at
	`, runID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScoringRow{}
	for rows.Next() {
		var r ScoringRow
		if err := rows.Scan(&r.RuleID, &r.Severity, &r.Effective, &r.ReviewStatus, &r.AssessmentStatus); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PendingReviewCount returns how many findings are still UNREVIEWED — the
// finalize precondition (全部处置后才能出报告).
func (d *DB) PendingReviewCount(ctx context.Context, actor auth.Actor, runID uuid.UUID) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `
		SELECT count(*) FROM findings
		WHERE audit_run_id = $1 AND organization_id = $2 AND review_status = 'UNREVIEWED'
	`, runID, actor.OrganizationID).Scan(&n)
	return n, err
}

// FinalizeRun atomically freezes the report: findings must all be
// dispositioned and the caller passes the computed scoring snapshot; the run
// becomes COMPLETED and read-only afterwards (R-06). Guarded to the actor's
// organization and non-terminal status.
func (d *DB) FinalizeRun(ctx context.Context, actor auth.Actor, runID uuid.UUID, scoringJSON []byte) error {
	tag, err := d.Pool.Exec(ctx, `
		UPDATE audit_runs SET status = 'COMPLETED', scoring = $2, completed_at = $3
		WHERE id = $1 AND organization_id = $4
		  AND status IN ('QUEUED','INGESTING','ANALYZING','REVIEW_REQUIRED')
	`, runID, scoringJSON, time.Now(), actor.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrJobConflict
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mustUUID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err) // uuid.NewV7 only fails on entropy failure; unrecoverable
	}
	return id
}

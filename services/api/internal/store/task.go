package store

// Fix task state machine (B11) — explicit transitions per contracts/state-machines.md §4.

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

var ErrIllegalTransition = errors.New("illegal workflow transition")

// legalTransitions per state-machines.md §4 (from → allowed to-set).
// RESOLVED is terminal: a later retest failure creates a NEW finding/task
// instead of resurrecting this one (历史不被覆盖, R-06).
var legalTransitions = map[string]map[string]bool{
	"OPEN":             {"ACKNOWLEDGED": true, "ACCEPTED_RISK": true},
	"ACKNOWLEDGED":     {"FIXING": true, "ACCEPTED_RISK": true},
	"FIXING":           {"READY_FOR_RETEST": true, "ACCEPTED_RISK": true},
	"READY_FOR_RETEST": {"RESOLVED": true, "REOPENED": true},
	"REOPENED":         {"FIXING": true, "READY_FOR_RETEST": true, "ACCEPTED_RISK": true},
}

type FixTask struct {
	ID             uuid.UUID `json:"id"`
	FindingID      uuid.UUID `json:"finding_id"`
	WorkflowStatus string    `json:"workflow_status"`
	Version        int       `json:"version"`
}

// EnsureFixTask creates the OPEN task for a finding on first demand.
func (d *DB) EnsureFixTask(ctx context.Context, actor auth.Actor, findingID uuid.UUID) (FixTask, error) {
	var t FixTask
	err := d.Pool.QueryRow(ctx, `
		SELECT id, workflow_status, version FROM fix_tasks
		WHERE finding_id = $1 AND organization_id = $2
	`, findingID, actor.OrganizationID).Scan(&t.ID, &t.WorkflowStatus, &t.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		id, err := uuid.NewV7()
		if err != nil {
			return FixTask{}, err
		}
		_, ierr := d.Pool.Exec(ctx, `
			INSERT INTO fix_tasks (id, finding_id, organization_id, owner_id) VALUES ($1,$2,$3,$4)
		`, id, findingID, actor.OrganizationID, actor.UserID)
		if ierr != nil {
			return FixTask{}, ierr
		}
		return FixTask{ID: id, FindingID: findingID, WorkflowStatus: "OPEN", Version: 1}, nil
	}
	t.FindingID = findingID
	return t, err
}

// TransitionFixTask validates the transition, applies it with optimistic
// locking and records the event row — one transaction. Human "改好了" only
// reaches READY_FOR_RETEST; RESOLVED requires the retest evidence (B11
// verify: 人工点击不直接产生 RESOLVED). ACCEPTED_RISK requires a reason and
// is never equivalent to PASS (ADR-0002).
func (d *DB) TransitionFixTask(ctx context.Context, actor auth.Actor, findingID uuid.UUID, to, reason, requestID string) (FixTask, error) {
	existing, err := d.EnsureFixTask(ctx, actor, findingID)
	if err != nil {
		return FixTask{}, err
	}
	current := existing.WorkflowStatus
	if !legalTransitions[current][to] {
		return FixTask{}, fmt.Errorf("%w: %s → %s", ErrIllegalTransition, current, to)
	}
	if to == "ACCEPTED_RISK" && reason == "" {
		return FixTask{}, errors.New("ACCEPTED_RISK requires a reason")
	}

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return FixTask{}, err
	}
	defer tx.Rollback(ctx)

	var newVersion int
	err = tx.QueryRow(ctx, `
		UPDATE fix_tasks SET workflow_status = $3, reason = $4,
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND workflow_status = $2
		RETURNING version
	`, existing.ID, current, to, reason).Scan(&newVersion)
	if err != nil {
		return FixTask{}, err
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return FixTask{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fix_task_events (id, task_id, actor_id, from_status, to_status, reason, request_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, eventID, existing.ID, actor.UserID, current, to, reason, requestID); err != nil {
		return FixTask{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return FixTask{}, err
	}
	existing.WorkflowStatus = to
	existing.Version = newVersion
	return existing, nil
}

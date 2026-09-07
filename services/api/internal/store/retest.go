package store

// Retest (B12): a new run pointing at its parent (R-07); history is never
// overwritten. Diff aligns findings by stable rule key.

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

// CreateRetestRun creates a child run of parentRunID with the given evidence
// (explicit selection — 绝不默认替换旧对象), reusing CreateAuditWithJob and
// then linking parent_run_id in the same flow.
func (d *DB) CreateRetestRun(ctx context.Context, actor auth.Actor, parentRunID uuid.UUID, evidenceIDs []uuid.UUID, standardVersion, rulesSHA256 string) (AuditRun, error) {
	parent, err := d.GetRun(ctx, actor, parentRunID)
	if err != nil {
		return AuditRun{}, err
	}
	if parent.Status != "COMPLETED" && parent.Status != "REVIEW_REQUIRED" {
		return AuditRun{}, errors.New("retest requires a COMPLETED or REVIEW_REQUIRED parent run")
	}
	child, err := d.CreateAuditWithJob(ctx, actor, parent.ProjectID, CreateAuditInput{
		EvidenceIDs:     evidenceIDs,
		StandardVersion: standardVersion,
		RulesSHA256:     rulesSHA256,
	})
	if err != nil {
		return AuditRun{}, err
	}
	if _, err := d.Pool.Exec(ctx,
		`UPDATE audit_runs SET parent_run_id = $2 WHERE id = $1`, child.ID, parentRunID); err != nil {
		return AuditRun{}, err
	}
	return d.GetRun(ctx, actor, child.ID)
}

// DiffEntry aligns parent and child findings by stable rule key (R-07).
type DiffEntry struct {
	RuleID     string `json:"rule_id"`
	Parent     string // parent effective/assessment status, "" when absent
	Child      string // child assessment status, "" when absent
	Comparable bool
}

// DiffAgainstParent compares the run's findings with its parent's by rule_id.
// A missing finding in the child is "not assessed" — NEVER auto-PASS (R-07).
func (d *DB) DiffAgainstParent(ctx context.Context, actor auth.Actor, runID uuid.UUID) ([]DiffEntry, string, error) {
	child, err := d.GetRun(ctx, actor, runID)
	if err != nil {
		return nil, "", err
	}
	if child.ParentRunID == nil {
		return nil, "run has no parent to compare", nil
	}
	parentStatus := map[string]string{}
	rows, err := d.Pool.Query(ctx, `
		SELECT rule_id, COALESCE(effective_status, assessment_status)
		FROM findings WHERE audit_run_id = $1
	`, *child.ParentRunID)
	if err != nil {
		return nil, "", err
	}
	for rows.Next() {
		var rule, status string
		if err := rows.Scan(&rule, &status); err != nil {
			rows.Close()
			return nil, "", err
		}
		parentStatus[rule] = status
	}
	rows.Close()

	childStatus := map[string]string{}
	childRows, err := d.Pool.Query(ctx, `
		SELECT rule_id, COALESCE(effective_status, assessment_status)
		FROM findings WHERE audit_run_id = $1
	`, runID)
	if err != nil {
		return nil, "", err
	}
	defer childRows.Close()
	for childRows.Next() {
		var rule, status string
		if err := childRows.Scan(&rule, &status); err != nil {
			return nil, "", err
		}
		childStatus[rule] = status
	}
	if err := childRows.Err(); err != nil {
		return nil, "", err
	}

	entries := []DiffEntry{}
	seen := map[string]bool{}
	for rule, child := range childStatus {
		seen[rule] = true
		entries = append(entries, DiffEntry{
			RuleID: rule, Parent: parentStatus[rule], Child: child,
			Comparable: parentStatus[rule] != "",
		})
	}
	for rule := range parentStatus {
		if !seen[rule] {
			entries = append(entries, DiffEntry{
				RuleID: rule, Parent: parentStatus[rule], Child: "",
				Comparable: true,
			})
		}
	}
	note := ""
	if len(entries) == 0 {
		note = "no findings on either side"
	}
	return entries, note, nil
}

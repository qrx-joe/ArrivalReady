package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

// BuildAuditPayload returns the BuildPayload function for Worker: it assembles
// the contracts/json-schema/job_payload envelope from the claimed job's frozen
// manifest, the project profile and the standard snapshot (rulesPath is the
// bound rules.yaml resolved at boot).
//
// Contract note (B08 follow-up): evidence CONTENT transfer (inline bytes vs
// presigned URLs) is deliberately not decided here — the B07 payload carries
// manifest metadata only, because the real assessment pipeline lands in B08
// together with that contract amendment.
func BuildAuditPayload(db *store.DB, rulesPath string) func(context.Context, *store.ClaimedJob) ([]byte, error) {
	return func(ctx context.Context, claimed *store.ClaimedJob) ([]byte, error) {
		var run struct {
			ProjectID    uuid.UUID
			EntityType   string
			TargetLocale string
		}
		if err := db.Pool.QueryRow(ctx, `
			SELECT p.id, p.entity_type, p.target_locale
			FROM audit_runs r JOIN projects p ON p.id = r.project_id
			WHERE r.id = $1
		`, claimed.RunID).Scan(&run.ProjectID, &run.EntityType, &run.TargetLocale); err != nil {
			return nil, fmt.Errorf("load run project: %w", err)
		}

		_, _, std, err := LoadStandard(rulesPath)
		if err != nil {
			return nil, fmt.Errorf("load bound rules: %w", err)
		}

		rules := make([]map[string]any, 0, len(std.Rules))
		for _, r := range std.Rules {
			if !applicable(r.Applicability.EntityTypes, r.Applicability.TargetLocales, run.EntityType, run.TargetLocale) {
				continue
			}
			rules = append(rules, map[string]any{
				"code":             r.Code,
				"dimension":        r.Dimension,
				"description":      r.Description,
				"severity_default": r.SeverityDefault,
				"review_required":  false,
			})
		}

		evidence := make([]map[string]any, 0, len(claimed.Manifest))
		for _, m := range claimed.Manifest {
			e, err := db.GetEvidenceInternal(ctx, m.EvidenceID)
			if err != nil {
				return nil, fmt.Errorf("manifest evidence %s: %w", m.EvidenceID, err)
			}
			evidence = append(evidence, map[string]any{
				"evidence_id":       e.ID,
				"project_id":        e.ProjectID,
				"type":              e.Type,
				"source_uri":        e.SourceNote,
				"object_key":        e.ObjectKey,
				"sha256":            e.SHA256,
				"mime_type":         e.MimeType,
				"journey_stage":     e.JourneyStage,
				"processing_status": e.ProcessingStatus,
				"captured_at":       e.CapturedAt,
			})
		}

		return json.Marshal(map[string]any{
			"job_id":          claimed.ID,
			"run_id":          claimed.RunID,
			"attempt_token":   *claimed.AttemptToken,
			"idempotency_key": fmt.Sprintf("run-%s/assessment/attempt-%d", claimed.RunID, claimed.Attempts),
			"project": map[string]any{
				"project_id":    run.ProjectID,
				"entity_type":   run.EntityType,
				"target_locale": run.TargetLocale,
			},
			"standard": map[string]any{
				"code":         "IRRS",
				"version":      claimed.StandardVersion,
				"rules_sha256": claimed.RulesSHA256,
			},
			"evidence":         evidence,
			"applicable_rules": rules,
			"budget":           map[string]any{"max_provider_requests": 6, "timeout_ms": 120000},
		})
	}
}

func applicable(entityTypes, locales []string, entityType, targetLocale string) bool {
	if !contains(entityTypes, entityType) && !contains(entityTypes, "other") {
		return false
	}
	if contains(locales, "*") || contains(locales, localeLanguage(targetLocale)) {
		return true
	}
	return len(locales) == 0
}

func localeLanguage(tag string) string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == '-' {
			return tag[:i]
		}
	}
	return tag
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

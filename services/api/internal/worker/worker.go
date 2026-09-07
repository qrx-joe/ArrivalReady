// Package worker owns the audit job lifecycle (execution plan B07, R-11):
// claim with SKIP LOCKED, lease + attempt token, bounded retries, cancellation
// checks, grounding before persistence, and atomic result writes. The Python
// AI service never touches business tables: it answers a ProviderResponse and
// THIS package decides what is persisted.
package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/storage"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

// AIClient is the transport boundary to the Python AI service. Network and
// HTTP-level errors are RETRYABLE; a well-formed ProviderResponse carries its
// own retryability verdict.
type AIClient interface {
	Assess(ctx context.Context, payload []byte) (status int, body []byte, err error)
}

type HTTPAIClient struct {
	BaseURL string
	HTTP    *http.Client
}

func (c *HTTPAIClient) Assess(ctx context.Context, payload []byte) (int, []byte, error) {
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/internal/assess", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// Worker executes claimed jobs. All fields are required except Lease/Interval
// which default sensibly.
type Worker struct {
	DB           *store.DB
	AI           AIClient
	RulesPath    string // bound rules.yaml; hash must match the run's snapshot
	Storage      storage.Store
	Lease        time.Duration
	Interval     time.Duration
	BuildPayload func(ctx context.Context, claimed *store.ClaimedJob) ([]byte, error)
}

// Run loops until ctx is cancelled. Errors from single jobs are contained:
// a poison job must not stop the queue.
func (w *Worker) Run(ctx context.Context) {
	interval := w.Interval
	if interval == 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		_ = w.ProcessOne(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessOne runs claim → assess → persist for at most one job.
func (w *Worker) ProcessOne(ctx context.Context) error {
	claimed, err := w.DB.ClaimNextJob(ctx, w.Lease)
	if err != nil {
		return fmt.Errorf("claim: %w", err)
	}
	if claimed == nil {
		return nil
	}
	return w.process(ctx, claimed)
}

func (w *Worker) process(ctx context.Context, claimed *store.ClaimedJob) error {
	token := *claimed.AttemptToken

	payload, err := w.BuildPayload(ctx, claimed)
	if err != nil {
		// Payload construction uses only local, deterministic inputs; a
		// failure here repeats identically on retry, so fail definitively.
		_, _ = w.DB.FailJobAttempt(ctx, claimed.ID, token, "payload build: "+err.Error())
		return nil
	}

	status, body, err := w.AI.Assess(ctx, payload)
	if err != nil {
		// Transport failures are retryable (TECH_SPEC §28): back to QUEUED.
		return w.DB.ReleaseJobAttempt(ctx, claimed.ID, token, "transport: "+err.Error())
	}
	if status != http.StatusOK {
		return w.DB.ReleaseJobAttempt(ctx, claimed.ID, token, fmt.Sprintf("ai service http %d", status))
	}

	outcome, perr := parseProviderResponse(body)
	if perr != nil {
		_, _ = w.DB.FailJobAttempt(ctx, claimed.ID, token, "invalid provider response: "+perr.Error())
		return nil
	}
	// The response must belong to THIS attempt; a mismatch means our lease
	// view is stale — discard without persisting (contract §6.3).
	if outcome.Metadata.AttemptToken != token.String() {
		_, _ = w.DB.FailJobAttempt(ctx, claimed.ID, token, "provider response attempt_token mismatch")
		return nil
	}

	if outcome.Outcome == "failure" {
		if outcome.Error.Retryable {
			return w.DB.ReleaseJobAttempt(ctx, claimed.ID, token, "provider: "+outcome.Error.Code+": "+outcome.Error.Message)
		}
		_, _ = w.DB.FailJobAttempt(ctx, claimed.ID, token, "provider: "+outcome.Error.Code+": "+outcome.Error.Message)
		return nil
	}

	findings, gerr := Ground(outcome, claimed.Manifest, boundRules(payload))
	if gerr != nil {
		_, _ = w.DB.FailJobAttempt(ctx, claimed.ID, token, "grounding: "+gerr.Error())
		return nil
	}
	if _, err := w.DB.CompleteJobAttempt(ctx, claimed.ID, token, findings, ""); err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	return nil
}

// ---- ProviderResponse types (mirror contracts/json-schema) ----

type providerResponse struct {
	Outcome     string               `json:"outcome"`
	Metadata    providerMetadata     `json:"metadata"`
	Assessments []providerAssessment `json:"assessments"`
	Error       *providerError       `json:"error"`
	// Attempts carries per-call usage metadata; its contents are not needed
	// for persistence, but the field must exist or DisallowUnknownFields
	// rejects every contract-valid response.
	Attempts json.RawMessage `json:"attempts"`
}

type providerMetadata struct {
	JobID         string  `json:"job_id"`
	RunID         string  `json:"run_id"`
	AttemptToken  string  `json:"attempt_token"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	ModelVersion  *string `json:"model_version"`
	PromptVersion string  `json:"prompt_version"`
	SchemaVersion string  `json:"schema_version"`
	InputHash     string  `json:"input_hash"`
	RequestedAt   string  `json:"requested_at"`
}

type providerAssessment struct {
	RuleID           string        `json:"rule_id"`
	Status           string        `json:"status"`
	Severity         string        `json:"severity"`
	EvidenceRefs     []providerRef `json:"evidence_refs"`
	Observation      string        `json:"observation"`
	Reason           string        `json:"reason"`
	RecommendedFix   string        `json:"recommended_fix"`
	Confidence       float64       `json:"confidence"`
	NeedsHumanReview bool          `json:"needs_human_review"`
}

type providerRef struct {
	EvidenceID string          `json:"evidence_id"`
	Locator    json.RawMessage `json:"locator"`
}

type providerError struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message"`
}

func parseProviderResponse(body []byte) (*providerResponse, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields() // schema drift is a contract violation, not noise
	var out providerResponse
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	if out.Outcome != "success" && out.Outcome != "failure" {
		return nil, fmt.Errorf("unknown outcome %q", out.Outcome)
	}
	if out.Outcome == "failure" && out.Error == nil {
		return nil, errors.New("failure outcome without error object")
	}
	return &out, nil
}

// Ground converts provider assessments into persistent findings, dropping
// anything that fails grounding (R-10/B08 安全失败: 无效引用不入正式 Finding):
//   - rule must exist in the bound rule set sent with the payload;
//   - PASS/WARN/FAIL must cite ≥1 evidence from THIS run's manifest;
//   - status/severity enums and confidence range are re-checked.
//
// A hard error (nil findings, err != nil) is reserved for structural
// impossibilities; individual bad assessments are dropped with a count.
func Ground(out *providerResponse, manifest []store.ManifestEntry, rules map[string]bool) ([]store.PersistentFinding, error) {
	if out.Outcome != "success" {
		return nil, errors.New("not a success outcome")
	}
	manifestIDs := make(map[string]bool, len(manifest))
	for _, m := range manifest {
		manifestIDs[m.EvidenceID.String()] = true
	}

	findings := make([]store.PersistentFinding, 0, len(out.Assessments))
	dropped := 0
	for _, a := range out.Assessments {
		if !rules[a.RuleID] {
			dropped++
			continue // unknown rule: never persisted
		}
		switch a.Status {
		case "PASS", "WARN", "FAIL", "UNKNOWN":
		default:
			dropped++
			continue
		}
		switch a.Severity {
		case "S0", "S1", "S2", "S3", "Info":
		default:
			dropped++
			continue
		}
		if a.Confidence < 0 || a.Confidence > 1 {
			dropped++
			continue
		}
		// US-03: a non-UNKNOWN conclusion without evidence cannot exist.
		if a.Status != "UNKNOWN" && len(a.EvidenceRefs) == 0 {
			dropped++
			continue
		}
		f := store.PersistentFinding{
			RuleID:         a.RuleID,
			Status:         a.Status,
			Severity:       a.Severity,
			Observation:    a.Observation,
			Reason:         a.Reason,
			RecommendedFix: a.RecommendedFix,
			Confidence:     a.Confidence,
		}
		raw, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		f.Original = raw
		if a.Status != "UNKNOWN" {
			for _, ref := range a.EvidenceRefs {
				if !manifestIDs[ref.EvidenceID] {
					continue // cross-run or fabricated reference: drop the link
				}
				f.EvidenceRefs = append(f.EvidenceRefs, store.EvidenceLink{
					EvidenceID: uuid.MustParse(ref.EvidenceID),
					Locator:    ref.Locator,
				})
			}
			if len(f.EvidenceRefs) == 0 {
				// Every citation was invalid → the conclusion loses its basis.
				dropped++
				continue
			}
		}
		findings = append(findings, f)
	}
	if dropped > 0 && len(findings) == 0 && len(out.Assessments) > 0 {
		return nil, fmt.Errorf("all %d assessments failed grounding", dropped)
	}
	return findings, nil
}

// boundRules re-parses the payload's applicable_rules so grounding validates
// against exactly what was sent (never against the live file).
func boundRules(payload []byte) map[string]bool {
	var p struct {
		ApplicableRules []struct {
			Code string `json:"code"`
		} `json:"applicable_rules"`
	}
	_ = json.Unmarshal(payload, &p)
	set := make(map[string]bool, len(p.ApplicableRules))
	for _, r := range p.ApplicableRules {
		set[r.Code] = true
	}
	return set
}

// ---- rules file access (shared with audit creation) ----

type StandardFile struct {
	Standard struct {
		Version string `yaml:"version"`
		Status  string `yaml:"status"`
	} `yaml:"standard"`
	Rules []struct {
		Code            string `yaml:"code"`
		Dimension       string `yaml:"dimension"`
		Description     string `yaml:"description"`
		SeverityDefault string `yaml:"severity_default"`
		Status          string `yaml:"status"`
		Applicability   struct {
			EntityTypes   []string `yaml:"entity_types"`
			TargetLocales []string `yaml:"target_locales"`
		} `yaml:"applicability"`
	} `yaml:"rules"`
}

// LoadStandard reads the rules file, returning its version, sha256 and rules.
func LoadStandard(path string) (version, shaHex string, std *StandardFile, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", nil, err
	}
	sum := sha256.Sum256(raw)
	shaHex = hex.EncodeToString(sum[:])

	// YAML parse via gopkg.in/yaml.v3 — imported here to keep one loader.
	var file StandardFile
	if err := yamlUnmarshal(raw, &file); err != nil {
		return "", "", nil, err
	}
	if file.Standard.Version == "" || len(file.Rules) == 0 {
		return "", "", nil, errors.New("rules file missing version or rules")
	}
	return file.Standard.Version, shaHex, &file, nil
}

// ResolveStandardsPath finds the active rules.yaml the same way tests and
// cmd/migrate resolve the migrations directory (layout-relative with override).
func ResolveStandardsPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	candidates := []string{
		filepath.Join("..", "..", "standards", "irrs", "0.1.0", "rules.yaml"),
		filepath.Join("..", "..", "..", "..", "standards", "irrs", "0.1.0", "rules.yaml"),
		filepath.Join(".", "standards", "irrs", "0.1.0", "rules.yaml"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("rules.yaml not found; set STANDARDS_RULES_PATH or keep the repository layout")
}

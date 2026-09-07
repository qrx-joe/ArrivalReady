// Package scoring implements deterministic Readiness Score calculation per
// ADR-0002 (D-018, PO-approved 2026-09-07).
//
// Invariants (docs/03 §6.3):
//   - Scores derive ONLY from findings bound to the run's frozen standard
//     snapshot — never from the currently active version (R-03).
//   - UNKNOWN findings are excluded from BOTH numerator and denominator and
//     surface as coverage; never rewarded, never punished.
//   - NA shrinks the applicable set (post-review, with reason); UNKNOWN
//     cannot. The distinction changes the denominator by design.
//   - The total score is published ONLY when every applicable item is
//     reviewed and none is UNKNOWN; otherwise TotalScore stays nil and the
//     report is flagged partial — a partial score must never be presented
//     as a verdict (ADR-0002 §2.3).
//
// Hand-calc acceptance (ADR-0002 §3, pinned by unit tests): weights 2/1/1
// with PASS/WARN/FAIL → 62.50; UNKNOWN on the last → 83.33 with
// TotalScore=nil; NA on the last → 83.33 with a publishable total.
package scoring

import (
	"fmt"
	"math"
)

// DimensionWeights are the IRRS dimension weights (PRD §11.1, MVP 假设,
// frozen by run.standard.version for historical runs).
var DimensionWeights = map[string]float64{
	"D1": 0.10, "D2": 0.20, "D3": 0.15,
	"D4": 0.20, "D5": 0.15, "D6": 0.10, "D7": 0.10,
}

// Item is one applicable check item after human disposition.
type Item struct {
	Dimension string
	Weight    float64
	// Effective status after review: PASS/WARN/FAIL/UNKNOWN/NA.
	// UNKNOWN = honest unknown; NA = excluded by review with reason.
	Effective string
	Reviewed  bool // a human disposition exists
}

type DimensionScore struct {
	Dimension        string  `json:"dimension"`
	Score            float64 `json:"score"`
	EvaluatedWeight  float64 `json:"evaluated_weight"`
	ApplicableWeight float64 `json:"applicable_weight"`
}

type Report struct {
	Dimensions  []DimensionScore `json:"dimensions"`
	TotalScore  *float64         `json:"total_score"` // nil = 部分评估/无可评项
	Partial     bool             `json:"partial"`
	CoveragePct float64          `json:"coverage_pct"`
}

// evaluated reports whether the status participates in the score sums
// (PASS/WARN/FAIL do; UNKNOWN and NA are excluded by design).
func value(s string) (float64, bool, error) {
	switch s {
	case "PASS":
		return 1, true, nil
	case "WARN":
		return 0.5, true, nil
	case "FAIL":
		return 0, true, nil
	case "UNKNOWN", "NA":
		return 0, false, nil
	default:
		return 0, false, fmt.Errorf("unknown effective status %q", s)
	}
}

// Compute aggregates per ADR-0002 §2.2–§2.4. Rounding happens once, at the
// end (ROUND_HALF_UP at 2dp approximated by the int64 trick).
func Compute(items []Item) (Report, error) {
	dimEval := map[string]float64{}
	dimEvalW := map[string]float64{}
	dimAppW := map[string]float64{}
	totalEval, totalApp := 0.0, 0.0
	partial := false

	for _, it := range items {
		v, evaluated, err := value(it.Effective)
		if err != nil {
			return Report{}, err
		}
		dimAppW[it.Dimension] += it.Weight
		totalApp += it.Weight

		isNA := it.Effective == "NA"
		if isNA {
			// NA removes the item from the applicable set (分母缩小),
			// mirroring dimAppW adjustments below.
			dimAppW[it.Dimension] -= it.Weight
			totalApp -= it.Weight
			continue
		}
		if !it.Reviewed {
			partial = true // 未处置项压低覆盖率
			continue
		}
		if evaluated { // PASS/WARN/FAIL
			dimEval[it.Dimension] += it.Weight * v
			dimEvalW[it.Dimension] += it.Weight
			totalEval += it.Weight
		} else {
			partial = true // UNKNOWN：诚实未知，压低覆盖率（不惩罚不奖励）
		}
	}

	report := Report{Partial: partial, Dimensions: []DimensionScore{}}
	publishable := !partial && totalApp > 0 && totalEval == totalApp
	total := 0.0
	participatingDimWeight := 0.0
	for dim, dimWeight := range DimensionWeights {
		applicable := dimAppW[dim]
		if applicable == 0 {
			continue // 整维 N/A → 排除并重新归一化（分母同步缩小）
		}
		evaluated := dimEvalW[dim]
		score := 0.0
		if evaluated > 0 {
			score = 100 * dimEval[dim] / evaluated
		}
		ds := DimensionScore{
			Dimension: dim, Score: round2(score),
			EvaluatedWeight: round2(evaluated), ApplicableWeight: round2(applicable),
		}
		report.Dimensions = append(report.Dimensions, ds)
		if publishable {
			total += ds.Score * dimWeight
			participatingDimWeight += dimWeight
		}
	}
	if publishable {
		// 总分 = Σ(维度分 × 维度权重) / Σ(参评维度权重)（ADR-0002 §2.3）：
		// 分母归一化让整维 N/A 不改变其余维度的贡献。
		total = total / participatingDimWeight
	}
	if publishable {
		report.TotalScore = ptr(round2(total))
	}
	if totalApp > 0 {
		report.CoveragePct = round2(totalEval / totalApp * 100)
	}
	sortDimensions(report.Dimensions)
	return report, nil
}

func sortDimensions(ds []DimensionScore) {
	for i := 1; i < len(ds); i++ {
		for j := i; j > 0 && ds[j].Dimension < ds[j-1].Dimension; j-- {
			ds[j], ds[j-1] = ds[j-1], ds[j]
		}
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func ptr(v float64) *float64   { return &v }

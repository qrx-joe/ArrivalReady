package scoring

import (
	"math"
	"testing"
)

func f(dim string, weight float64, effective string, reviewed bool) Item {
	return Item{Dimension: dim, Weight: weight, Effective: effective, Reviewed: reviewed}
}

// ADR-0002 §3 手算验收例 — the numbers the PO approved.
func TestHandCalcAcceptance(t *testing.T) {
	items := []Item{
		f("D4", 2, "PASS", true),
		f("D4", 1, "WARN", true),
		f("D4", 1, "FAIL", true),
	}
	report, err := Compute(items)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Dimensions[0].Score; got != 62.5 {
		t.Fatalf("case 1: score = %v, want 62.5", got)
	}
	if report.TotalScore == nil || *report.TotalScore != 62.5 {
		t.Fatalf("case 1: total = %v, want 62.5", report.TotalScore)
	}
	if report.Partial {
		t.Fatal("case 1: fully reviewed must not be partial")
	}

	// 情形 2：第三项 UNKNOWN → 部分分 83.33、覆盖率 75%、总分不出。
	items[2] = f("D4", 1, "UNKNOWN", true)
	report, _ = Compute(items)
	if report.Dimensions[0].Score != 83.33 {
		t.Fatalf("case 2: score = %v, want 83.33", report.Dimensions[0].Score)
	}
	if report.TotalScore != nil || !report.Partial {
		t.Fatalf("case 2: total must be nil (partial), got %v", report.TotalScore)
	}
	if report.CoveragePct != 75 {
		t.Fatalf("case 2: coverage = %v, want 75", report.CoveragePct)
	}

	// 情形 3：第三项经 NA 排除 → 适用集缩小，可出总分。
	items[2] = f("D4", 1, "NA", true)
	report, _ = Compute(items)
	if report.TotalScore == nil || *report.TotalScore != 83.33 {
		t.Fatalf("case 3: total = %v, want 83.33", report.TotalScore)
	}
	if report.Partial {
		t.Fatal("case 3: NA-excluded fully reviewed must not be partial")
	}
}

func TestUnknownNeverScores(t *testing.T) {
	report, err := Compute([]Item{f("D2", 3, "UNKNOWN", true)})
	if err != nil {
		t.Fatal(err)
	}
	if report.Dimensions[0].Score != 0 {
		t.Fatalf("UNKNOWN-only dimension must score 0, got %v", report.Dimensions[0].Score)
	}
	if report.TotalScore != nil {
		t.Fatal("UNKNOWN-only must not publish a total")
	}
	if report.CoveragePct != 0 {
		t.Fatalf("coverage = %v, want 0", report.CoveragePct)
	}
}

func TestAllNAYieldsNoScore(t *testing.T) {
	report, err := Compute([]Item{f("D5", 2, "NA", true)})
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalScore != nil {
		t.Fatalf("all-NA must not produce a score, got %v", *report.TotalScore)
	}
	if len(report.Dimensions) != 0 {
		t.Fatalf("all-NA dimension must be excluded, got %d", len(report.Dimensions))
	}
}

func TestS1FailNotHiddenByAverage(t *testing.T) {
	// S0/S1 FAIL 独立清单由调用方基于 findings 渲染；这里验证的是
	// 高均分场景：一项 FAIL 会让分数如实下降，不能被其他 PASS 稀释成满分。
	report, err := Compute([]Item{
		f("D3", 1, "FAIL", true),
		f("D3", 3, "PASS", true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Dimensions[0].Score; math.Abs(got-75) > 0.01 {
		t.Fatalf("score = %v, want 75 (FAIL drags honestly)", got)
	}
}

package mas

import "testing"

func TestEvaluateQuality(t *testing.T) {
	report := EvaluateQuality("交付速度要快，模块依赖要清晰，用指标和阈值复盘风险，并补充测试清单。")
	if report.Score != report.MaxScore {
		t.Fatalf("score=%d, want %d", report.Score, report.MaxScore)
	}
}

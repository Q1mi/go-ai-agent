package eval

import (
	"context"

	"github.com/q1mi/traceagent/internal/agent"
	"github.com/q1mi/traceagent/internal/obs"
)

// RunDataset 运行评估集并返回报告。
func RunDataset(ctx context.Context, samples []Sample, bot *agent.Agent, exporter *obs.MemoryExporter, evaluator Evaluator) ([]Report, error) {
	reports := make([]Report, 0, len(samples))
	for _, sample := range samples {
		result, err := bot.Run(ctx, sample.Input)
		if err != nil {
			return nil, err
		}
		score, err := evaluator.Evaluate(ctx, sample, result.Answer)
		if sample.Blocked {
			score = Score{Pass: result.Blocked, Value: boolFloat(result.Blocked), Reason: "安全拦截符合预期"}
			err = nil
		}
		if err != nil {
			return nil, err
		}
		spans := traceSpans(exporter, result.TraceID)
		reports = append(reports, Report{
			Sample:          sample,
			Output:          result.Answer,
			TraceID:         result.TraceID,
			TraceURL:        obs.TraceURL(result.TraceID),
			ResultScore:     score,
			TrajectoryScore: JudgeTrajectory(sample, result, spans),
		})
	}
	return reports, nil
}

func traceSpans(exporter *obs.MemoryExporter, traceID string) []obs.SpanRecord {
	if exporter == nil {
		return nil
	}
	var out []obs.SpanRecord
	for _, span := range exporter.Spans() {
		if span.TraceID == traceID {
			out = append(out, span)
		}
	}
	return out
}

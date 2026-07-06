package ctxeng

import (
	"context"
	"fmt"

	"github.com/q1mi/ctxagent/internal/llm"
)

// SummarizeFunc 把较早历史压缩为摘要。
type SummarizeFunc func(ctx context.Context, older []llm.Message) (string, error)

// Compact 把较早消息总结为一条摘要，保留 system 与最近 keepRecent 条原文。
func Compact(ctx context.Context, messages []llm.Message, keepRecent int, summarize SummarizeFunc) ([]llm.Message, error) {
	if len(messages) == 0 {
		return nil, nil
	}
	if keepRecent < 0 {
		keepRecent = 0
	}
	if summarize == nil {
		return nil, fmt.Errorf("summarize 不能为空")
	}
	if len(messages) <= keepRecent+1 {
		return messages, nil
	}

	system := messages[0]
	older := messages[1 : len(messages)-keepRecent]
	recent := messages[len(messages)-keepRecent:]
	summary, err := summarize(ctx, older)
	if err != nil {
		return nil, err
	}

	out := make([]llm.Message, 0, 2+keepRecent)
	out = append(out, system)
	out = append(out, llm.Message{
		Role:    llm.RoleUser,
		Name:    "compaction_summary",
		Content: "【早前对话摘要，据此延续】\n" + summary,
	})
	out = append(out, recent...)
	return out, nil
}

// AssembleConfig 注入上下文治理依赖。
type AssembleConfig struct {
	Budget     Budget
	KeepRecent int
	Summarize  SummarizeFunc
}

// AssembleReport 记录组装过程中的治理动作。
type AssembleReport struct {
	BeforeTokens int
	AfterTokens  int
	Compactions  int
	OverBefore   map[string]int
	OverAfter    map[string]int
}

// Assemble 组装一次调用的消息，历史超预算时自动压缩。
func Assemble(ctx context.Context, messages []llm.Message, cfg AssembleConfig) ([]llm.Message, error) {
	out, _, err := AssembleWithReport(ctx, messages, cfg)
	return out, err
}

// AssembleWithReport 组装消息并返回治理报告。
func AssembleWithReport(ctx context.Context, messages []llm.Message, cfg AssembleConfig) ([]llm.Message, AssembleReport, error) {
	report := AssembleReport{}
	history := llm.JoinContent(messages)
	report.BeforeTokens = EstimateTokens(history)
	report.OverBefore = cfg.Budget.Over("", "", history, "")

	for cfg.Budget.History > 0 && EstimateTokens(history) > cfg.Budget.History {
		compacted, err := Compact(ctx, messages, cfg.KeepRecent, cfg.Summarize)
		if err != nil {
			return nil, report, err
		}
		if len(compacted) >= len(messages) {
			break
		}
		messages = compacted
		report.Compactions++
		history = llm.JoinContent(messages)
	}

	report.AfterTokens = EstimateTokens(history)
	report.OverAfter = cfg.Budget.Over("", "", history, "")
	return messages, report, nil
}

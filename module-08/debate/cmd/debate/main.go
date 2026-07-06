package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/q1mi/debate/internal/llm"
	"github.com/q1mi/debate/internal/mas"
	"github.com/q1mi/debate/internal/providers/offline"
	"github.com/q1mi/debate/internal/providers/openai"
)

const defaultQuestion = "我们准备用单体架构起步、后期再拆微服务，这个决策有哪些风险？"

type options struct {
	question       string
	rounds         int
	provider       string
	model          string
	pragmaticModel string
	cautiousModel  string
	dataModel      string
	judgeModel     string
	transcriptPath string
	timeout        time.Duration
	compare        bool
}

type runStats struct {
	calls   int
	usage   llm.Usage
	elapsed time.Duration
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "[错误]", err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string, out io.Writer) error {
	opt, err := parseOptions(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, opt.timeout)
	defer cancel()

	baseProvider, providerName, err := buildProvider(opt.provider)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "问题：%s\n", opt.question)
	fmt.Fprintf(out, "Provider：%s\n", providerName)
	fmt.Fprintf(out, "轮数：%d\n\n", opt.rounds)

	var baseline string
	var baselineStats runStats
	if opt.compare {
		baselineProvider := llm.NewMeter(baseProvider)
		start := time.Now()
		baseline, err = mas.Baseline(ctx, baselineProvider, opt.model, opt.question)
		if err != nil {
			return err
		}
		baselineStats = snapshot(baselineProvider, time.Since(start))
		printSection(out, "单 Agent 直接回答", baseline)
		printStats(out, baselineStats)
		fmt.Fprintln(out)
	}

	debateProvider := llm.NewMeter(baseProvider)
	debaters := buildDebaters(debateProvider, opt)
	start := time.Now()
	transcript, err := mas.DebateWithTranscript(ctx, debaters, opt.question, opt.rounds)
	if err != nil {
		return err
	}
	final, err := mas.Judge(ctx, debateProvider, opt.judgeModelOrDefault(), opt.question, transcript.FinalAnswers)
	if err != nil {
		return err
	}
	debateStats := snapshot(debateProvider, time.Since(start))

	printTranscript(out, transcript)
	printSection(out, "评审定稿", final)
	printStats(out, debateStats)

	if opt.compare {
		fmt.Fprintln(out)
		printComparison(out, baselineStats, debateStats)
		fmt.Fprintln(out)
		printQualityComparison(out, baseline, final)
	}
	if opt.transcriptPath != "" {
		if err := writeTranscript(opt.transcriptPath, providerName, transcript, final, baseline, baselineStats, debateStats); err != nil {
			return err
		}
		fmt.Fprintf(out, "\nTranscript 已写入：%s\n", opt.transcriptPath)
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	opt := options{}
	fs := flag.NewFlagSet("debate", flag.ContinueOnError)
	fs.StringVar(&opt.question, "question", "", "辩论问题；为空时读取位置参数或使用默认问题")
	fs.IntVar(&opt.rounds, "rounds", 3, "辩论轮数")
	fs.StringVar(&opt.provider, "provider", "auto", "Provider：auto、offline、openai")
	fs.StringVar(&opt.model, "model", strings.TrimSpace(os.Getenv("LLM_MODEL")), "默认模型名称")
	fs.StringVar(&opt.pragmaticModel, "pragmatic-model", "", "务实派模型；为空时使用 -model")
	fs.StringVar(&opt.cautiousModel, "cautious-model", "", "谨慎派模型；为空时使用 -model")
	fs.StringVar(&opt.dataModel, "data-model", "", "数据派模型；为空时使用 -model")
	fs.StringVar(&opt.judgeModel, "judge-model", "", "评审模型；为空时使用 -model")
	fs.StringVar(&opt.transcriptPath, "transcript", "", "可选：把完整辩论记录写入 Markdown 文件")
	fs.DurationVar(&opt.timeout, "timeout", 2*time.Minute, "总超时时间")
	fs.BoolVar(&opt.compare, "compare", true, "同时运行单 Agent baseline")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if opt.question == "" {
		opt.question = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if opt.question == "" {
		opt.question = defaultQuestion
	}
	if opt.rounds <= 0 {
		return options{}, fmt.Errorf("rounds 必须大于 0")
	}
	if opt.timeout <= 0 {
		return options{}, fmt.Errorf("timeout 必须大于 0")
	}
	return opt, nil
}

func buildProvider(kind string) (llm.Provider, string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "auto":
		if strings.TrimSpace(os.Getenv("LLM_MODEL")) != "" {
			provider, err := openai.NewFromEnv()
			if err == nil {
				return provider, provider.Name(), nil
			}
		}
		provider := offline.New()
		return provider, provider.Name(), nil
	case "offline":
		provider := offline.New()
		return provider, provider.Name(), nil
	case "openai":
		provider, err := openai.NewFromEnv()
		if err != nil {
			return nil, "", err
		}
		return provider, provider.Name(), nil
	default:
		return nil, "", fmt.Errorf("未知 provider %q，可选值：auto、offline、openai", kind)
	}
}

func buildDebaters(provider llm.Provider, opt options) []mas.Debater {
	return []mas.Debater{
		{
			Name:     "务实派",
			Provider: provider,
			Model:    firstNonEmpty(opt.pragmaticModel, opt.model),
			Persona:  "你务实，关注落地速度、团队现状、工程成本和阶段性交付。",
		},
		{
			Name:     "谨慎派",
			Provider: provider,
			Model:    firstNonEmpty(opt.cautiousModel, opt.model),
			Persona:  "你谨慎，关注长期可维护性、技术债、故障隔离和组织协作风险。",
		},
		{
			Name:     "数据派",
			Provider: provider,
			Model:    firstNonEmpty(opt.dataModel, opt.model),
			Persona:  "你重证据，倾向使用事实、指标、案例和可验证阈值来支持判断。",
		},
	}
}

func (opt options) judgeModelOrDefault() string {
	return firstNonEmpty(opt.judgeModel, opt.model)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func snapshot(meter *llm.Meter, elapsed time.Duration) runStats {
	calls, usage := meter.Snapshot()
	return runStats{calls: calls, usage: usage, elapsed: elapsed}
}

func printTranscript(out io.Writer, transcript mas.Transcript) {
	for _, round := range transcript.Rounds {
		fmt.Fprintf(out, "=== 第 %d 轮 ===\n", round.Number)
		for _, answer := range round.Answers {
			fmt.Fprintf(out, "\n【%s】\n%s\n", answer.Name, answer.Content)
		}
		fmt.Fprintln(out)
	}
}

func printSection(out io.Writer, title, body string) {
	fmt.Fprintf(out, "=== %s ===\n%s\n", title, strings.TrimSpace(body))
}

func printStats(out io.Writer, stats runStats) {
	fmt.Fprintf(out, "\n调用次数：%d\n", stats.calls)
	fmt.Fprintf(out, "Token：prompt=%d completion=%d total=%d\n", stats.usage.PromptTokens, stats.usage.CompletionTokens, stats.usage.TotalTokens)
	fmt.Fprintf(out, "耗时：%s\n", stats.elapsed.Round(time.Millisecond))
}

func printComparison(out io.Writer, baseline, debate runStats) {
	fmt.Fprintln(out, "=== 成本对比 ===")
	fmt.Fprintln(out, "| 模式 | 调用次数 | Prompt Token | Completion Token | Total Token | 耗时 |")
	fmt.Fprintln(out, "|---|---:|---:|---:|---:|---:|")
	fmt.Fprintf(out, "| 单 Agent | %d | %d | %d | %d | %s |\n",
		baseline.calls, baseline.usage.PromptTokens, baseline.usage.CompletionTokens, baseline.usage.TotalTokens, baseline.elapsed.Round(time.Millisecond))
	fmt.Fprintf(out, "| Debate + Judge | %d | %d | %d | %d | %s |\n",
		debate.calls, debate.usage.PromptTokens, debate.usage.CompletionTokens, debate.usage.TotalTokens, debate.elapsed.Round(time.Millisecond))
}

func printQualityComparison(out io.Writer, baseline, debateFinal string) {
	baselineReport := mas.EvaluateQuality(baseline)
	debateReport := mas.EvaluateQuality(debateFinal)
	fmt.Fprintln(out, "=== 质量覆盖对比 ===")
	fmt.Fprintln(out, "| 维度 | 单 Agent | Debate + Judge |")
	fmt.Fprintln(out, "|---|---:|---:|")
	for i := range baselineReport.Dimensions {
		base := mark(baselineReport.Dimensions[i].Hit)
		debate := mark(debateReport.Dimensions[i].Hit)
		fmt.Fprintf(out, "| %s | %s | %s |\n", baselineReport.Dimensions[i].Name, base, debate)
	}
	fmt.Fprintf(out, "| 合计 | %d/%d | %d/%d |\n",
		baselineReport.Score, baselineReport.MaxScore, debateReport.Score, debateReport.MaxScore)
}

func mark(hit bool) string {
	if hit {
		return "✓"
	}
	return "—"
}

func writeTranscript(path, providerName string, transcript mas.Transcript, final, baseline string, baselineStats, debateStats runStats) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Debate Transcript\n\n")
	fmt.Fprintf(&sb, "- Provider: %s\n", providerName)
	fmt.Fprintf(&sb, "- Question: %s\n\n", transcript.Question)
	if strings.TrimSpace(baseline) != "" {
		fmt.Fprintf(&sb, "## Single Agent Baseline\n\n%s\n\n", baseline)
		appendStats(&sb, baselineStats)
	}
	for _, round := range transcript.Rounds {
		fmt.Fprintf(&sb, "## Round %d\n\n", round.Number)
		for _, answer := range round.Answers {
			fmt.Fprintf(&sb, "### %s\n\n%s\n\n", answer.Name, answer.Content)
		}
	}
	fmt.Fprintf(&sb, "## Judge Final\n\n%s\n\n", final)
	appendStats(&sb, debateStats)
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func appendStats(sb *strings.Builder, stats runStats) {
	fmt.Fprintf(sb, "- Calls: %d\n", stats.calls)
	fmt.Fprintf(sb, "- Prompt Tokens: %d\n", stats.usage.PromptTokens)
	fmt.Fprintf(sb, "- Completion Tokens: %d\n", stats.usage.CompletionTokens)
	fmt.Fprintf(sb, "- Total Tokens: %d\n", stats.usage.TotalTokens)
	fmt.Fprintf(sb, "- Elapsed: %s\n\n", stats.elapsed.Round(time.Millisecond))
}

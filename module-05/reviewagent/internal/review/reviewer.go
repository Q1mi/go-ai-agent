package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/q1mi/reviewagent/internal/llm"
	"github.com/q1mi/reviewagent/internal/patterns"
)

const (
	defaultMaxRounds = 2
	outputMarkdown   = "markdown"
	outputJSON       = "json"
)

// Reviewer 是 M05 配套练习的小型代码审查 Agent。
type Reviewer struct {
	Provider     llm.Provider
	Model        string
	MaxRounds    int
	OutputFormat string // markdown 或 json
}

// AnswerOrReview 使用 IntentRouter 做前置分流。
//
// 输入被路由为 code_review 时进入三 Agent 审查流程；输入被路由为 general 时普通回答。
func (reviewer Reviewer) AnswerOrReview(ctx context.Context, input string) (string, error) {
	if reviewer.Provider == nil {
		return "", fmt.Errorf("Provider 不能为空")
	}
	router := patterns.IntentRouter{
		Provider: reviewer.Provider,
		Model:    reviewer.Model,
		Routes: []patterns.Route{
			{
				Name:        "code_review",
				Description: "用户输入包含 Go 代码，或明确要求审查、分析、改进 Go 代码",
				Handle: func(ctx context.Context, input string) (string, error) {
					report, err := reviewer.Review(ctx, input)
					if err != nil {
						return "", err
					}
					return reviewer.render(report)
				},
			},
			{
				Name:        "general",
				Description: "用户输入是普通问题、解释请求、闲聊，或没有提供代码",
				Handle:      reviewer.answerGeneral,
			},
		},
		Fallback: func(ctx context.Context, input string) (string, error) {
			if LooksLikeGoCode(input) {
				report, err := reviewer.Review(ctx, input)
				if err != nil {
					return "", err
				}
				return reviewer.render(report)
			}
			return reviewer.answerGeneral(ctx, input)
		},
	}
	return router.Dispatch(ctx, input)
}

// Review 执行 Planner → Generator → Evaluator 的三 Agent 范式。
func (reviewer Reviewer) Review(ctx context.Context, code string) (Report, error) {
	if reviewer.Provider == nil {
		return Report{}, fmt.Errorf("Provider 不能为空")
	}
	plan, err := planReview(ctx, reviewer.Provider, reviewer.Model, code)
	if err != nil {
		return Report{}, err
	}
	maxRounds := reviewer.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}

	rounds := 0
	generator := func(ctx context.Context, feedback string) (string, error) {
		rounds++
		report, err := reviewer.generateReport(ctx, code, plan, feedback)
		if err != nil {
			return "", err
		}
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	evaluator := func(ctx context.Context, output string) (patterns.Evaluation, error) {
		return reviewer.evaluateReport(ctx, code, plan, output)
	}

	output, evaluation, err := patterns.EvaluatorOptimizer(ctx, generator, evaluator, maxRounds)
	if err != nil {
		return Report{}, err
	}
	report, err := llm.ParseInto[Report](output)
	if err != nil {
		return Report{}, fmt.Errorf("解析最终报告失败: %w", err)
	}
	report.Evaluation = normalizeEvaluation(evaluation)
	report.Rounds = rounds
	return report, nil
}

func planReview(ctx context.Context, provider llm.Provider, model, code string) (Plan, error) {
	system := "你是资深 Go 代码审查 Planner。分析代码，列出最值得审查的 3-5 个维度，" +
		"严格只输出 JSON：{\"dimensions\":[\"正确性\",\"错误处理\",\"并发安全\"]}。"
	out, err := patterns.Complete(ctx, provider, model, system, code)
	if err != nil {
		return Plan{}, err
	}
	plan, err := llm.ParseInto[Plan](out)
	if err != nil {
		return Plan{}, fmt.Errorf("Planner 输出无法解析: %w", err)
	}
	plan.Dimensions = normalizeDimensions(plan.Dimensions)
	if len(plan.Dimensions) == 0 {
		return Plan{}, fmt.Errorf("Planner 未产出审查维度")
	}
	return plan, nil
}

func (reviewer Reviewer) generateReport(ctx context.Context, code string, plan Plan, feedback string) (Report, error) {
	tasks := make([]func(context.Context) (string, error), len(plan.Dimensions))
	for i, dimension := range plan.Dimensions {
		dimension := dimension
		tasks[i] = func(ctx context.Context) (string, error) {
			finding, err := reviewer.reviewDimension(ctx, code, dimension, feedback)
			if err != nil {
				return "", err
			}
			raw, err := json.Marshal(finding)
			if err != nil {
				return "", err
			}
			return string(raw), nil
		}
	}
	results, err := patterns.Sectioning(ctx, tasks)
	if err != nil {
		return Report{}, err
	}
	findings := make([]Finding, 0, len(results))
	for _, result := range results {
		finding, err := llm.ParseInto[Finding](result)
		if err != nil {
			return Report{}, fmt.Errorf("解析维度审查结果失败: %w", err)
		}
		findings = append(findings, normalizeFinding(finding))
	}
	return Report{
		Plan:     plan,
		Findings: findings,
	}, nil
}

func (reviewer Reviewer) reviewDimension(ctx context.Context, code string, dimension string, feedback string) (Finding, error) {
	system := fmt.Sprintf(
		"你是资深 Go 代码审查 Generator。只审查维度：%s。"+
			"输出必须具体、可操作。严格只输出 JSON："+
			"{\"dimension\":\"%s\",\"severity\":\"info|low|medium|high\",\"location\":\"...\",\"problem\":\"...\",\"evidence\":\"...\",\"suggestion\":\"...\"}。",
		dimension,
		dimension,
	)
	var user strings.Builder
	fmt.Fprintf(&user, "待审查代码：\n```go\n%s\n```\n", code)
	if strings.TrimSpace(feedback) != "" {
		fmt.Fprintf(&user, "\nEvaluator 反馈，请重点补漏：\n%s\n", feedback)
	}
	out, err := patterns.Complete(ctx, reviewer.Provider, reviewer.Model, system, user.String())
	if err != nil {
		return Finding{}, fmt.Errorf("维度 %q 审查失败: %w", dimension, err)
	}
	finding, err := llm.ParseInto[Finding](out)
	if err != nil {
		return Finding{}, fmt.Errorf("维度 %q 输出无法解析: %w", dimension, err)
	}
	if strings.TrimSpace(finding.Dimension) == "" {
		finding.Dimension = dimension
	}
	return normalizeFinding(finding), nil
}

func (reviewer Reviewer) evaluateReport(ctx context.Context, code string, plan Plan, reportJSON string) (patterns.Evaluation, error) {
	system := "你是代码审查报告 Evaluator。对照原始代码和审查维度，判断报告是否充分。" +
		"评估标准：是否覆盖计划维度、发现是否具体、建议是否可执行、是否存在明显漏报。" +
		"严格只输出 JSON：{\"pass\":true,\"score\":90,\"feedback\":\"...\"}。"
	dimensions, _ := json.Marshal(plan.Dimensions)
	user := "审查维度：\n" + string(dimensions) +
		"\n\n原始代码：\n```go\n" + code +
		"\n```\n\n待评估报告 JSON：\n" + reportJSON
	out, err := patterns.Complete(ctx, reviewer.Provider, reviewer.Model, system, user)
	if err != nil {
		return patterns.Evaluation{}, err
	}
	evaluation, err := llm.ParseInto[patterns.Evaluation](out)
	if err != nil {
		return patterns.Evaluation{}, fmt.Errorf("Evaluator 输出无法解析: %w", err)
	}
	return normalizeEvaluation(evaluation), nil
}

func (reviewer Reviewer) answerGeneral(ctx context.Context, input string) (string, error) {
	system := "你是一个简洁的 Go 课程助手。用户没有提供需要审查的 Go 代码时，直接回答问题。"
	return patterns.Complete(ctx, reviewer.Provider, reviewer.Model, system, input)
}

func (reviewer Reviewer) render(report Report) (string, error) {
	switch strings.ToLower(strings.TrimSpace(reviewer.OutputFormat)) {
	case "", outputMarkdown:
		return report.Markdown(), nil
	case outputJSON:
		return report.JSON()
	default:
		return "", fmt.Errorf("未知输出格式 %q，可选 markdown 或 json", reviewer.OutputFormat)
	}
}

func normalizeDimensions(dimensions []string) []string {
	seen := make(map[string]bool, len(dimensions))
	out := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		dimension = strings.TrimSpace(dimension)
		if dimension == "" || seen[dimension] {
			continue
		}
		seen[dimension] = true
		out = append(out, dimension)
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func normalizeFinding(finding Finding) Finding {
	finding.Dimension = strings.TrimSpace(finding.Dimension)
	finding.Severity = normalizeSeverity(finding.Severity)
	finding.Location = strings.TrimSpace(finding.Location)
	finding.Problem = strings.TrimSpace(finding.Problem)
	finding.Evidence = strings.TrimSpace(finding.Evidence)
	finding.Suggestion = strings.TrimSpace(finding.Suggestion)
	return finding
}

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info", "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	case "严重", "高", "高危":
		return "high"
	case "中", "中等":
		return "medium"
	case "低":
		return "low"
	default:
		return "info"
	}
}

func normalizeEvaluation(evaluation patterns.Evaluation) patterns.Evaluation {
	if evaluation.Score < 0 {
		evaluation.Score = 0
	}
	if evaluation.Score > 100 {
		evaluation.Score = 100
	}
	evaluation.Feedback = strings.TrimSpace(evaluation.Feedback)
	if !evaluation.Pass && evaluation.Feedback == "" {
		evaluation.Feedback = "报告仍需补充更具体的问题、证据和修改建议。"
	}
	return evaluation
}

// LooksLikeGoCode 是 IntentRouter 失败时使用的本地兜底判断。
func LooksLikeGoCode(input string) bool {
	text := strings.TrimSpace(input)
	if text == "" {
		return false
	}
	markers := []string{
		"package ",
		"func ",
		"type ",
		"import ",
		"interface{",
		"interface {",
		"chan ",
		"go func",
		"defer ",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return strings.Contains(text, "{") && strings.Contains(text, "}") && strings.Contains(text, "(")
}

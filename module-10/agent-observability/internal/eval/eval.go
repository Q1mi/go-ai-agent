package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/q1mi/traceagent/internal/agent"
	"github.com/q1mi/traceagent/internal/llm"
	"github.com/q1mi/traceagent/internal/obs"
)

// Sample 是一条评估样本。
type Sample struct {
	ID            string   `json:"id"`
	Input         string   `json:"input"`
	Expected      string   `json:"expected"`
	Keywords      []string `json:"keywords"`
	RequiredTools []string `json:"required_tools"`
	Blocked       bool     `json:"blocked,omitempty"`
}

// Score 是统一评分结构。
type Score struct {
	Pass   bool    `json:"pass"`
	Value  float64 `json:"value"`
	Reason string  `json:"reason"`
}

// Evaluator 是评估器接口。
type Evaluator interface {
	Name() string
	Evaluate(ctx context.Context, sample Sample, output string) (Score, error)
}

// ContainsAll 检查回答是否包含所有关键词。
type ContainsAll struct{}

// Name 返回评估器名称。
func (ContainsAll) Name() string { return "contains_all" }

// Evaluate 执行确定性检查。
func (ContainsAll) Evaluate(_ context.Context, sample Sample, output string) (Score, error) {
	for _, kw := range sample.Keywords {
		if !strings.Contains(output, kw) {
			return Score{Pass: false, Value: 0, Reason: "缺少关键词: " + kw}, nil
		}
	}
	return Score{Pass: true, Value: 1, Reason: "包含全部关键词"}, nil
}

// LightJudge 使用模型做轻量语义评估。
type LightJudge struct {
	Provider llm.Provider
	Model    string
}

// Name 返回评估器名称。
func (LightJudge) Name() string { return "light_judge" }

// Evaluate 执行轻量判官评估。
func (judge LightJudge) Evaluate(ctx context.Context, sample Sample, output string) (Score, error) {
	if judge.Provider == nil {
		return Score{}, fmt.Errorf("provider 不能为空")
	}
	prompt := "判断【实际回答】是否正确回应了【问题】并与【参考答案】一致。只输出 JSON：{\"pass\":bool,\"reason\":\"\"}\n\n" +
		"问题：" + sample.Input + "\n参考答案：" + sample.Expected + "\n实际回答：" + output
	resp, err := judge.Provider.Chat(ctx, llm.ChatRequest{
		Model: judge.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "你是严格的评估员。"},
			{Role: llm.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return Score{}, err
	}
	parsed, err := llm.ParseJSON[struct {
		Pass   bool   `json:"pass"`
		Reason string `json:"reason"`
	}](resp.Content)
	if err != nil {
		return Score{}, err
	}
	return Score{Pass: parsed.Pass, Value: boolFloat(parsed.Pass), Reason: parsed.Reason}, nil
}

// TrajectoryScore 是轨迹评估四维分。
type TrajectoryScore struct {
	TaskCompletion    float64 `json:"task_completion"`
	StepEfficiency    float64 `json:"step_efficiency"`
	ToolAccuracy      float64 `json:"tool_accuracy"`
	ActionAdvancement float64 `json:"action_advancement"`
	Reason            string  `json:"reason"`
}

// Average 返回均分。
func (score TrajectoryScore) Average() float64 {
	return (score.TaskCompletion + score.StepEfficiency + score.ToolAccuracy + score.ActionAdvancement) / 4
}

// JudgeTrajectory 用执行结果和 trace 评估轨迹质量。
func JudgeTrajectory(sample Sample, result agent.Result, spans []obs.SpanRecord) TrajectoryScore {
	taskCompletion := 0.0
	if sample.Blocked && result.Blocked {
		taskCompletion = 1
	} else {
		pass, _ := ContainsAll{}.Evaluate(context.Background(), sample, result.Answer)
		taskCompletion = pass.Value
	}

	stepEfficiency := 1.0
	if result.Steps > 4 {
		stepEfficiency = 0.5
	}
	if result.Steps == 0 && !result.Blocked {
		stepEfficiency = 0
	}

	toolAccuracy := requiredToolScore(sample.RequiredTools, result.ToolCalls)
	if sample.Blocked {
		toolAccuracy = 1
	}

	actionAdvancement := 0.0
	if result.Blocked {
		actionAdvancement = 1
	} else if hasSpan(spans, "chat ") && hasSpan(spans, "execute_tool ") && strings.TrimSpace(result.Answer) != "" {
		actionAdvancement = 1
	}

	return TrajectoryScore{
		TaskCompletion:    taskCompletion,
		StepEfficiency:    stepEfficiency,
		ToolAccuracy:      toolAccuracy,
		ActionAdvancement: actionAdvancement,
		Reason:            reason(taskCompletion, stepEfficiency, toolAccuracy, actionAdvancement),
	}
}

// Report 是单样本评估报告。
type Report struct {
	Sample          Sample          `json:"sample"`
	Output          string          `json:"output"`
	TraceID         string          `json:"trace_id"`
	TraceURL        string          `json:"trace_url"`
	ResultScore     Score           `json:"result_score"`
	TrajectoryScore TrajectoryScore `json:"trajectory_score"`
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func requiredToolScore(required []string, calls []agent.ToolCall) float64 {
	if len(required) == 0 {
		if len(calls) == 0 {
			return 1
		}
		return 0.8
	}
	seen := map[string]bool{}
	for _, call := range calls {
		seen[call.Name] = true
	}
	hit := 0
	for _, name := range required {
		if seen[name] {
			hit++
		}
	}
	return float64(hit) / float64(len(required))
}

func hasSpan(spans []obs.SpanRecord, prefix string) bool {
	for _, span := range spans {
		if strings.HasPrefix(span.Name, prefix) {
			return true
		}
	}
	return false
}

func reason(task, steps, tools, action float64) string {
	var parts []string
	if task < 1 {
		parts = append(parts, "任务完成度不足")
	}
	if steps < 1 {
		parts = append(parts, "步骤效率偏低")
	}
	if tools < 1 {
		parts = append(parts, "工具选择未完全命中")
	}
	if action < 1 {
		parts = append(parts, "轨迹缺少有效推进")
	}
	if len(parts) == 0 {
		return "轨迹完成目标，工具和步骤合理"
	}
	return strings.Join(parts, "；")
}

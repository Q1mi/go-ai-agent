package patterns

import (
	"context"
	"fmt"
	"strings"

	"github.com/q1mi/reviewagent/internal/llm"
)

// Worker 是 Orchestrator-Workers 模式中子任务执行者的最小接口。
type Worker interface {
	Run(ctx context.Context, goal string) (string, error)
}

// WorkerFunc 让普通函数可以作为 Worker 使用。
type WorkerFunc func(ctx context.Context, goal string) (string, error)

// Run 执行子任务。
func (fn WorkerFunc) Run(ctx context.Context, goal string) (string, error) {
	return fn(ctx, goal)
}

type subtask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type decomposition struct {
	Subtasks []subtask `json:"subtasks"`
}

// Orchestrator 动态拆解任务、并行分发 Worker、再综合结果。
type Orchestrator struct {
	Provider  llm.Provider
	Model     string
	NewWorker func() Worker
}

// Run 执行 Orchestrator-Workers 流程。
func (orchestrator *Orchestrator) Run(ctx context.Context, goal string) (string, error) {
	if orchestrator.Provider == nil {
		return "", fmt.Errorf("orchestrator.Provider 不能为空")
	}
	if orchestrator.NewWorker == nil {
		return "", fmt.Errorf("orchestrator.NewWorker 不能为空")
	}
	system := "你是任务编排者。把用户任务拆解为若干可独立完成的子任务，" +
		"严格只输出 JSON：{\"subtasks\":[{\"id\":\"t1\",\"description\":\"...\"}]}"
	raw, err := Complete(ctx, orchestrator.Provider, orchestrator.Model, system, goal)
	if err != nil {
		return "", err
	}
	plan, err := llm.ParseInto[decomposition](raw)
	if err != nil {
		return "", fmt.Errorf("分解结果无法解析: %w", err)
	}
	if len(plan.Subtasks) == 0 {
		return "", fmt.Errorf("编排者未产出任何子任务")
	}

	tasks := make([]func(context.Context) (string, error), len(plan.Subtasks))
	for i, item := range plan.Subtasks {
		item := item
		tasks[i] = func(ctx context.Context) (string, error) {
			worker := orchestrator.NewWorker()
			if worker == nil {
				return "", fmt.Errorf("子任务 %s 未创建 worker", item.ID)
			}
			res, err := worker.Run(ctx, item.Description)
			if err != nil {
				return "", fmt.Errorf("子任务 %s 失败: %w", item.ID, err)
			}
			return fmt.Sprintf("[%s] %s\n%s", item.ID, item.Description, res), nil
		}
	}
	results, err := Sectioning(ctx, tasks)
	if err != nil {
		return "", err
	}

	synthSystem := "下面是各子任务的执行结果，请综合成对原始任务的完整、连贯回答。"
	return Complete(ctx, orchestrator.Provider, orchestrator.Model, synthSystem,
		"原始任务：\n"+goal+"\n\n子任务结果：\n"+strings.Join(results, "\n\n"))
}

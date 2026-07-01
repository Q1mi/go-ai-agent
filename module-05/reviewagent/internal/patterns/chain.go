package patterns

import (
	"context"
	"fmt"

	"github.com/q1mi/reviewagent/internal/llm"
)

// ChainStep 表示 Prompt Chaining 中的一步。
type ChainStep struct {
	Name   string                   // 步骤名，用于报错定位
	System string                   // 本步的系统提示词
	Build  func(prev string) string // 用上一步输出构造本步用户输入
	Gate   func(out string) error   // 可选校验，失败时中止后续步骤
}

// RunChain 顺序执行各步骤，前一步输出作为后一步输入。
func RunChain(ctx context.Context, provider llm.Provider, model, input string, steps []ChainStep) (string, error) {
	cur := input
	for _, step := range steps {
		if step.Build == nil {
			return "", fmt.Errorf("步骤 %q 缺少 Build", step.Name)
		}
		out, err := Complete(ctx, provider, model, step.System, step.Build(cur))
		if err != nil {
			return "", fmt.Errorf("步骤 %q 调用失败: %w", step.Name, err)
		}
		if step.Gate != nil {
			if err := step.Gate(out); err != nil {
				return "", fmt.Errorf("步骤 %q 校验未通过: %w", step.Name, err)
			}
		}
		cur = out
	}
	return cur, nil
}

package agent

import (
	"fmt"
	"time"
)

// Budget 定义 Agent 单轮运行的停止边界。
//
// MaxSteps 控制模型思考轮数，MaxTokens 控制累计 token，用于避免无限循环和成本失控。
type Budget struct {
	MaxSteps        int
	MaxTokens       int
	Deadline        time.Time
	MaxSameAction   int
	MaxHealAttempts int
}

// DefaultBudget 对应课件 4.6 的安全默认值。
// 练习 CLI 会通过 --max-steps 和 --max-tokens 覆盖其中两项。
func DefaultBudget() Budget {
	return Budget{
		MaxSteps:        10,
		MaxTokens:       12000,
		MaxSameAction:   3,
		MaxHealAttempts: 3,
	}
}

// Exceeded 对应课件 4.6 的停止条件检查。
// Agent 循环每轮开始时先调用它，避免无边界循环。
func (budget Budget) Exceeded(state *State) (bool, string) {
	if state == nil {
		return false, ""
	}
	if budget.MaxSteps > 0 && state.Step >= budget.MaxSteps {
		return true, fmt.Sprintf("达到最大步骤数 %d", budget.MaxSteps)
	}
	if budget.MaxTokens > 0 && state.tokenTotal() >= budget.MaxTokens {
		return true, fmt.Sprintf("达到 token 预算 %d", budget.MaxTokens)
	}
	if !budget.Deadline.IsZero() && time.Now().After(budget.Deadline) {
		return true, "达到运行截止时间"
	}
	return false, ""
}

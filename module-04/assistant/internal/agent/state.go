package agent

import (
	"time"

	"github.com/q1mi/assistant/internal/llm"
)

// Phase 表示 Agent 当前运行阶段。
type Phase string

const (
	// PhaseThinking 表示 Agent 正在请求模型或等待模型规划下一步。
	PhaseThinking Phase = "thinking"
	// PhaseActing 表示 Agent 正在执行工具。
	PhaseActing Phase = "acting"
	// PhaseDone 表示本轮任务已经完成。
	PhaseDone Phase = "done"
	// PhaseError 表示本轮任务失败。
	PhaseError Phase = "error"
)

// State 对应课件 4.2 的运行快照。
// Messages 是 Agent 的运行记忆；每一轮模型调用都会带上这份历史。
type State struct {
	Goal         string            `json:"goal"`
	Messages     []llm.Message     `json:"messages"`
	Step         int               `json:"step"`
	Phase        Phase             `json:"phase"`
	Answer       string            `json:"answer,omitempty"`
	Usage        llm.Usage         `json:"usage"`
	ActionCounts map[string]int    `json:"action_counts,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// tokenTotal 返回当前状态累计 token。
func (state *State) tokenTotal() int {
	if state == nil {
		return 0
	}
	return state.Usage.TotalTokens()
}

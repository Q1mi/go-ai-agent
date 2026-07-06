package offline

import (
	"context"
	"fmt"
	"strings"

	"github.com/q1mi/debate/internal/llm"
)

// Provider 是可离线运行的 deterministic provider，便于学员直接观察多 Agent 流程。
type Provider struct {
	name string
}

// New 创建离线 Provider。
func New() *Provider {
	return &Provider{name: "offline"}
}

// Name 返回 Provider 名称。
func (provider *Provider) Name() string {
	return provider.name
}

// Chat 根据 system persona 和 user prompt 生成稳定回答。
func (provider *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	system, user := splitMessages(req.Messages)
	content := provider.answer(system, user)
	usage := llm.Usage{
		PromptTokens:     llm.EstimatePromptTokens(req.Messages),
		CompletionTokens: llm.EstimateTokens(content),
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return &llm.ChatResponse{Content: content, Usage: usage}, nil
}

func splitMessages(messages []llm.Message) (string, string) {
	var systemParts []string
	var userParts []string
	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleSystem:
			systemParts = append(systemParts, msg.Content)
		case llm.RoleUser:
			userParts = append(userParts, msg.Content)
		}
	}
	return strings.Join(systemParts, "\n"), strings.Join(userParts, "\n")
}

func (provider *Provider) answer(system, user string) string {
	question := extractQuestion(user)
	revision := strings.Contains(user, "上一轮") || strings.Contains(user, "其他成员")

	switch {
	case strings.Contains(system, "评审"):
		return judgeAnswer(question, user)
	case strings.Contains(system, "务实"):
		return personaAnswer("务实派", question, revision, []string{
			"先保持可交付节奏，避免过早拆分造成沟通和部署成本。",
			"保留模块边界、接口契约和自动化测试，为后续演进留出空间。",
			"设置触发条件，例如团队规模、发布频率、性能瓶颈和数据边界。",
		})
	case strings.Contains(system, "谨慎"):
		return personaAnswer("谨慎派", question, revision, []string{
			"关注长期耦合风险，尤其是数据库共享、模块依赖和发布回滚。",
			"在单体阶段就建立分层、依赖方向和架构守护规则。",
			"提前识别未来拆分成本最高的上下文，避免核心域被横向逻辑污染。",
		})
	case strings.Contains(system, "数据") || strings.Contains(system, "证据"):
		return personaAnswer("数据派", question, revision, []string{
			"先收集当前事实：团队人数、峰值流量、发布频次、故障类型和变更热点。",
			"用指标决定拆分时机，例如构建耗时、变更冲突率、接口错误率和 MTTR。",
			"把方案转成实验：记录基线、设定阈值、定期复盘架构债务。",
		})
	default:
		return baselineAnswer(question)
	}
}

func extractQuestion(user string) string {
	for _, line := range strings.Split(user, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "问题：")
		line = strings.TrimPrefix(line, "任务：")
		if strings.TrimSpace(line) != "" && !strings.Contains(line, "其他成员") && !strings.Contains(line, "各方回答") {
			return strings.TrimSpace(line)
		}
	}
	return strings.TrimSpace(user)
}

func personaAnswer(name, question string, revision bool, points []string) string {
	var sb strings.Builder
	if revision {
		fmt.Fprintf(&sb, "%s修订观点：我保留核心判断，并吸收其他视角补齐风险边界。\n", name)
	} else {
		fmt.Fprintf(&sb, "%s初始观点：针对“%s”，我的判断如下。\n", name, question)
	}
	for i, point := range points {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, point)
	}
	if revision {
		sb.WriteString("补充：最终建议把决策写成可验证的阶段性承诺，定期用指标复盘。")
	}
	return strings.TrimSpace(sb.String())
}

func baselineAnswer(question string) string {
	return strings.TrimSpace(fmt.Sprintf(`单 Agent 直接回答：针对“%s”，建议以单体起步，同时保留演进空间。

1. 明确模块边界和依赖方向，减少后续拆分阻力。
2. 建立测试、监控和发布自动化，降低架构调整风险。
3. 定义拆分触发条件，例如团队规模、性能瓶颈、变更冲突和故障恢复时间。
4. 定期复盘技术债，避免单体长期失控。`, question))
}

func judgeAnswer(question, user string) string {
	return strings.TrimSpace(fmt.Sprintf(`评审定稿：针对“%s”，单体起步可以作为阶段策略，但需要把风险治理前置。

综合结论：
1. 短期优先交付速度，保持架构简单，降低协调和部署成本。
2. 单体内部必须建立清晰模块边界、依赖约束、接口契约和自动化测试。
3. 用数据定义演进阈值，包括发布频率、变更冲突率、构建耗时、故障恢复时间和性能瓶颈。
4. 每个迭代周期复盘架构债务，发现核心域耦合或团队协作受阻时启动拆分预案。

质量判断：辩论覆盖了落地速度、长期维护和数据验证三个维度；最终方案应写入可执行的检查清单。`, question))
}

package demo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/q1mi/ctxagent/internal/ctxeng"
	"github.com/q1mi/ctxagent/internal/llm"
	"github.com/q1mi/ctxagent/internal/tool"
)

const systemPrompt = `你是上下文治理演示 Agent。你需要围绕架构方案回答问题，必要时读取文档。回答保持简短，并保留关键事实、风险和待办。`

// Config 控制演示参数。
type Config struct {
	Rounds           int
	DocsDir          string
	MemoryDir        string
	Budget           ctxeng.Budget
	OffloadThreshold int
	KeepRecent       int
	MaxTools         int
}

// Result 是完整演示结果。
type Result struct {
	Points           []Point
	RawMessages      []llm.Message
	GovernedMessages []llm.Message
	MemoryIDs        []string
}

// Point 记录一轮治理前后的上下文 token。
type Point struct {
	Round           int
	UserInput       string
	RawTokens       int
	GovernedTokens  int
	RawHistory      int
	GovernedHistory int
	SelectedTools   []string
	Events          []string
}

// Run 执行未治理与已治理两条链路并返回对比结果。
func Run(ctx context.Context, cfg Config) (Result, error) {
	cfg = withDefaults(cfg)
	memory := &ctxeng.FileMemory{Dir: cfg.MemoryDir}
	allTools := buildTools(cfg.DocsDir, memory)
	allToolText := tool.Definitions(allTools)

	rawMessages := []llm.Message{{Role: llm.RoleSystem, Content: systemPrompt}}
	governedMessages := []llm.Message{{Role: llm.RoleSystem, Content: systemPrompt}}
	prompts := prompts(cfg.Rounds)
	result := Result{}

	for i, userInput := range prompts {
		round := i + 1
		rawMessages = append(rawMessages, llm.Message{Role: llm.RoleUser, Content: userInput})
		governedMessages = append(governedMessages, llm.Message{Role: llm.RoleUser, Content: userInput})

		var events []string
		selected := ctxeng.SelectTools(userInput, allTools, cfg.MaxTools)
		selected = ensureTool(selected, allTools, "read_doc")
		selectedText := tool.Definitions(selected)
		if len(selected) == 0 {
			selected = allTools
			selectedText = allToolText
		}

		rawToolResult, err := callReadDoc(ctx, allTools)
		if err != nil {
			return Result{}, err
		}
		rawMessages = append(rawMessages, llm.Message{Role: llm.RoleTool, Name: "read_doc", Content: rawToolResult})

		governedToolResult, err := callReadDoc(ctx, selected)
		if err != nil {
			return Result{}, err
		}
		placeholder, info, err := memory.OffloadWithInfo(governedToolResult, cfg.OffloadThreshold)
		if err != nil {
			return Result{}, err
		}
		if info.Offloaded {
			result.MemoryIDs = append(result.MemoryIDs, info.ID)
			events = append(events, fmt.Sprintf("offload read_doc: %d -> %d tokens id=%s", info.BeforeTokens, info.AfterTokens, info.ID))
		}
		governedMessages = append(governedMessages, llm.Message{Role: llm.RoleTool, Name: "read_doc", Content: placeholder})

		rawMessages = append(rawMessages, llm.Message{Role: llm.RoleAssistant, Content: answer(userInput, false)})
		governedMessages = append(governedMessages, llm.Message{Role: llm.RoleAssistant, Content: answer(userInput, true)})

		compacted, report, err := ctxeng.AssembleWithReport(ctx, governedMessages, ctxeng.AssembleConfig{
			Budget:     cfg.Budget,
			KeepRecent: cfg.KeepRecent,
			Summarize:  summarizeForDemo,
		})
		if err != nil {
			return Result{}, err
		}
		governedMessages = compacted
		if report.Compactions > 0 {
			events = append(events, fmt.Sprintf("compact history: %d -> %d tokens (%d 次)", report.BeforeTokens, report.AfterTokens, report.Compactions))
		}

		rawHistory := llm.JoinContent(rawMessages)
		governedHistory := llm.JoinContent(governedMessages)
		rawUsage := cfg.Budget.Count(systemPrompt, allToolText, rawHistory, "")
		governedUsage := cfg.Budget.Count(systemPrompt, selectedText, governedHistory, "")

		result.Points = append(result.Points, Point{
			Round:           round,
			UserInput:       userInput,
			RawTokens:       rawUsage.Total,
			GovernedTokens:  governedUsage.Total,
			RawHistory:      rawUsage.History,
			GovernedHistory: governedUsage.History,
			SelectedTools:   tool.Names(selected),
			Events:          events,
		})
	}

	result.RawMessages = rawMessages
	result.GovernedMessages = governedMessages
	return result, nil
}

func withDefaults(cfg Config) Config {
	if cfg.Rounds <= 0 {
		cfg.Rounds = 8
	}
	if strings.TrimSpace(cfg.DocsDir) == "" {
		cfg.DocsDir = "./docs"
	}
	if strings.TrimSpace(cfg.MemoryDir) == "" {
		cfg.MemoryDir = ".ctx-memory"
	}
	if cfg.Budget.History <= 0 {
		cfg.Budget.History = 900
	}
	if cfg.Budget.Tools <= 0 {
		cfg.Budget.Tools = 160
	}
	if cfg.Budget.Total <= 0 {
		cfg.Budget.Total = 1500
	}
	if cfg.OffloadThreshold <= 0 {
		cfg.OffloadThreshold = 260
	}
	if cfg.KeepRecent <= 0 {
		cfg.KeepRecent = 6
	}
	if cfg.MaxTools <= 0 {
		cfg.MaxTools = 2
	}
	return cfg
}

func buildTools(docsDir string, memory *ctxeng.FileMemory) []tool.Tool {
	return []tool.Tool{
		tool.New("read_doc", "读取 架构 文档 长文 方案 风险 architecture document", func(ctx context.Context, input string) (string, error) {
			path := strings.TrimSpace(input)
			if path == "" {
				path = "architecture-plan.md"
			}
			data, err := os.ReadFile(filepath.Join(docsDir, filepath.Base(path)))
			if err != nil {
				return "", err
			}
			return string(data), nil
		}),
		tool.New("read_memory", "读取 外置 内容 memory id 全文", func(ctx context.Context, input string) (string, error) {
			return memory.Read(input)
		}),
		tool.New("calc_risk_score", "计算 风险 分数 score 权重", func(ctx context.Context, input string) (string, error) {
			return "risk_score=72/100，主要风险来自数据耦合、发布回滚和团队协作。", nil
		}),
		tool.New("list_todos", "列出 待办 todo checklist 清单", func(ctx context.Context, input string) (string, error) {
			return "待办：补充模块边界图；定义拆分阈值；建立发布回滚演练。", nil
		}),
		tool.New("search_policy", "搜索 合规 规范 policy 安全", func(ctx context.Context, input string) (string, error) {
			return "规范摘要：涉及客户数据的模块需要保留审计日志和权限边界。", nil
		}),
		tool.New("create_ticket", "创建 工单 ticket issue 任务", func(ctx context.Context, input string) (string, error) {
			return "已创建演示工单 CTX-1001。", nil
		}),
	}
}

func callReadDoc(ctx context.Context, tools []tool.Tool) (string, error) {
	for _, item := range tools {
		if item.Name() == "read_doc" {
			return item.Call(ctx, "architecture-plan.md")
		}
	}
	return "", fmt.Errorf("当前工具集合中缺少 read_doc")
}

func ensureTool(selected, all []tool.Tool, name string) []tool.Tool {
	for _, item := range selected {
		if item.Name() == name {
			return selected
		}
	}
	for _, item := range all {
		if item.Name() == name {
			return append(selected, item)
		}
	}
	return selected
}

func prompts(rounds int) []string {
	base := []string{
		"请阅读架构方案文档，指出单体起步的主要风险。",
		"继续结合文档说明数据层和模块边界的风险。",
		"请给出风险分数，并列出前三个治理动作。",
		"从发布、回滚和故障隔离角度复盘这个方案。",
		"查一下是否涉及合规或审计要求。",
		"基于前面讨论生成待办清单。",
		"再次阅读方案，确认有没有遗漏的长期维护风险。",
		"请把最终建议压缩成适合汇报的结论。",
	}
	out := make([]string, 0, rounds)
	for len(out) < rounds {
		out = append(out, base[len(out)%len(base)])
	}
	return out
}

func answer(input string, governed bool) string {
	mode := "未治理"
	if governed {
		mode = "已治理"
	}
	return fmt.Sprintf("%s回答：已基于文档识别风险。建议保留模块边界、数据所有权、发布回滚和拆分触发阈值。当前问题：%s", mode, input)
}

func summarizeForDemo(ctx context.Context, older []llm.Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var facts []string
	var memIDs []string
	for _, msg := range older {
		if msg.Role == llm.RoleUser {
			facts = append(facts, firstRunes(msg.Content, 60))
		}
		for _, part := range strings.Fields(msg.Content) {
			if strings.HasPrefix(part, "id=mem-") {
				memIDs = append(memIDs, strings.TrimSuffix(strings.TrimPrefix(part, "id="), "，摘要:"))
			}
		}
	}
	if len(facts) > 5 {
		facts = facts[len(facts)-5:]
	}
	now := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("压缩时间：%s\n保留事实：%s\n外置引用：%s\n待办：继续围绕风险、指标和行动项回答。",
		now, strings.Join(facts, "；"), strings.Join(memIDs, ", ")), nil
}

func firstRunes(s string, n int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n]) + "…"
}

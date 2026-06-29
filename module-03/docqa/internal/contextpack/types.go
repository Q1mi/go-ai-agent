package contextpack

import "github.com/q1mi/docqa-context/internal/llm"

// Document 表示进入 prompt 上下文的一段资料。
type Document struct {
	ID      string
	Title   string
	Source  string
	Content string
}

// Example 表示 few-shot 示例中的一组问答。
type Example struct {
	Question string
	Answer   string
}

// Budget 表示文档问答场景中的上下文预算。
type Budget struct {
	Total        int
	SystemPrompt int
	ToolSchema   int
	History      int
	Retrieved    int
	UserInput    int
	Output       int
}

// SectionUsage 表示某个上下文部分的估算 token 和超限处理建议。
type SectionUsage struct {
	Name       string
	Tokens     int
	Limit      int
	CacheHint  string
	Overflow   bool
	Resolution string
}

// Plan 是一次文档问答请求的完整上下文方案。
//
// CLI 的 dry-run 会把这里的字段打印出来，真实调用会把 Messages 交给大模型网关。
type Plan struct {
	Product          string
	Question         string
	CurrentTime      string
	SystemPrompt     string
	DifficultySchema string
	Messages         []llm.Message
	Documents        []Document
	Budget           Budget
	Usages           []SectionUsage
	CachePrefix      []string
	TimePlacement    string
	LongDocsStrategy string
}

// DifficultyOutput 是演示结构化输出时使用的问题难度 Schema。
type DifficultyOutput struct {
	Level  string `json:"level" description:"问题难度，只能取 low、medium、high" enum:"low,medium,high"`
	Reason string `json:"reason" description:"用一句话说明判断依据"`
}

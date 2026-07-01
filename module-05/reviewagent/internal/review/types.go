package review

import "github.com/q1mi/reviewagent/internal/patterns"

// Plan 是 Planner 输出的结构化审查计划。
type Plan struct {
	Dimensions []string `json:"dimensions"`
}

// Finding 是某个审查维度下的一条发现。
type Finding struct {
	Dimension  string `json:"dimension"`
	Severity   string `json:"severity"`   // info / low / medium / high
	Location   string `json:"location"`   // 文件、函数或行号；未知时写“未定位”
	Problem    string `json:"problem"`    // 具体问题
	Evidence   string `json:"evidence"`   // 代码证据或推理依据
	Suggestion string `json:"suggestion"` // 可执行修改建议
}

// Report 是最终结构化代码审查报告。
type Report struct {
	Plan       Plan                `json:"plan"`
	Findings   []Finding           `json:"findings"`
	Evaluation patterns.Evaluation `json:"evaluation"`
	Rounds     int                 `json:"rounds"`
}

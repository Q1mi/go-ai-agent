package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSON 返回缩进后的结构化报告。
func (report Report) JSON() (string, error) {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Markdown 返回适合命令行阅读的审查报告。
func (report Report) Markdown() string {
	var builder strings.Builder
	builder.WriteString("# Go 代码审查报告\n\n")
	builder.WriteString("## 审查维度\n\n")
	for _, dimension := range report.Plan.Dimensions {
		fmt.Fprintf(&builder, "- %s\n", dimension)
	}
	builder.WriteString("\n## 发现\n\n")
	if len(report.Findings) == 0 {
		builder.WriteString("未发现明确问题。\n\n")
	} else {
		for i, finding := range report.Findings {
			fmt.Fprintf(&builder, "### %d. [%s] %s\n\n", i+1, finding.Severity, finding.Dimension)
			fmt.Fprintf(&builder, "- 位置：%s\n", fallback(finding.Location, "未定位"))
			fmt.Fprintf(&builder, "- 问题：%s\n", fallback(finding.Problem, "未发现明确问题"))
			fmt.Fprintf(&builder, "- 证据：%s\n", fallback(finding.Evidence, "无"))
			fmt.Fprintf(&builder, "- 建议：%s\n\n", fallback(finding.Suggestion, "无需修改"))
		}
	}
	builder.WriteString("## Evaluator 结论\n\n")
	fmt.Fprintf(&builder, "- 是否通过：%t\n", report.Evaluation.Pass)
	fmt.Fprintf(&builder, "- 分数：%d\n", report.Evaluation.Score)
	fmt.Fprintf(&builder, "- 反馈：%s\n", fallback(report.Evaluation.Feedback, "无"))
	fmt.Fprintf(&builder, "- 生成轮次：%d\n", report.Rounds)
	return builder.String()
}

func fallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

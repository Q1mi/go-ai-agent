package mas

import "strings"

// DimensionScore 表示答案在某个质量维度上的覆盖情况。
type DimensionScore struct {
	Name  string
	Hit   bool
	Terms []string
}

// QualityReport 是启发式质量覆盖报告。
type QualityReport struct {
	Dimensions []DimensionScore
	Score      int
	MaxScore   int
}

// EvaluateQuality 用关键词覆盖度粗略评估答案完整性，便于课程练习做 baseline 对比。
func EvaluateQuality(answer string) QualityReport {
	defs := []DimensionScore{
		{Name: "落地速度与成本", Terms: []string{"交付", "速度", "成本", "团队"}},
		{Name: "长期维护", Terms: []string{"维护", "技术债", "模块", "依赖"}},
		{Name: "风险治理", Terms: []string{"风险", "故障", "回滚", "隔离"}},
		{Name: "数据验证", Terms: []string{"指标", "数据", "阈值", "基线"}},
		{Name: "可执行性", Terms: []string{"触发", "清单", "复盘", "测试"}},
	}
	text := strings.ToLower(answer)
	report := QualityReport{MaxScore: len(defs)}
	for _, def := range defs {
		item := def
		for _, term := range def.Terms {
			if strings.Contains(text, strings.ToLower(term)) {
				item.Hit = true
				break
			}
		}
		if item.Hit {
			report.Score++
		}
		report.Dimensions = append(report.Dimensions, item)
	}
	return report
}

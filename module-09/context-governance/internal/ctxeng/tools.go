package ctxeng

import (
	"sort"
	"strings"
	"unicode"

	"github.com/q1mi/ctxagent/internal/tool"
)

// SelectTools 按 query 相关度选择最多 maxN 个工具，降低工具定义占用。
func SelectTools(query string, all []tool.Tool, maxN int) []tool.Tool {
	if maxN <= 0 || len(all) == 0 {
		return nil
	}
	type scored struct {
		index int
		tool  tool.Tool
		score int
	}
	queryTerms := terms(query)
	ranked := make([]scored, 0, len(all))
	for i, item := range all {
		text := strings.ToLower(item.Name() + " " + item.Description())
		score := 0
		for _, term := range queryTerms {
			if strings.Contains(text, term) {
				score++
			}
		}
		ranked = append(ranked, scored{index: i, tool: item, score: score})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].score > ranked[j].score
	})

	if maxN > len(ranked) {
		maxN = len(ranked)
	}
	out := make([]tool.Tool, 0, maxN)
	for i := 0; i < maxN; i++ {
		out = append(out, ranked[i].tool)
	}
	return out
}

func terms(text string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" || seen[term] {
			return
		}
		seen[term] = true
		out = append(out, term)
	}
	for _, term := range strings.Fields(text) {
		add(strings.Trim(term, " \t\n\r，。！？、:：;；,.()[]{}<>「」“”\"'"))
	}
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			add(string(r))
		}
	}
	return out
}

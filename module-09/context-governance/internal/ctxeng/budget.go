package ctxeng

// Budget 描述一次模型调用里各部分的 token 上限，0 表示不限。
type Budget struct {
	Total         int
	SystemPrompt  int
	Tools         int
	History       int
	Retrieved     int
	OutputReserve int
}

// Usage 记录一次上下文组装的估算 token。
type Usage struct {
	SystemPrompt int
	Tools        int
	History      int
	Retrieved    int
	Total        int
}

// Count 估算各部分 token。
func (b Budget) Count(systemPrompt, tools, history, retrieved string) Usage {
	usage := Usage{
		SystemPrompt: EstimateTokens(systemPrompt),
		Tools:        EstimateTokens(tools),
		History:      EstimateTokens(history),
		Retrieved:    EstimateTokens(retrieved),
	}
	usage.Total = usage.SystemPrompt + usage.Tools + usage.History + usage.Retrieved
	return usage
}

// Over 返回各部分超出预算的量，map 里出现即超标。
func (b Budget) Over(systemPrompt, tools, history, retrieved string) map[string]int {
	usage := b.Count(systemPrompt, tools, history, retrieved)
	over := map[string]int{}
	chk := func(name string, used, limit int) {
		if limit > 0 && used > limit {
			over[name] = used - limit
		}
	}
	chk("system", usage.SystemPrompt, b.SystemPrompt)
	chk("tools", usage.Tools, b.Tools)
	chk("history", usage.History, b.History)
	chk("retrieved", usage.Retrieved, b.Retrieved)
	if b.Total > 0 {
		available := b.Total - b.OutputReserve
		if available < 0 {
			available = 0
		}
		if usage.Total > available {
			over["total"] = usage.Total - available
		}
	}
	return over
}

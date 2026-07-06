package ctxeng

// Note 是 Agent 的外部工作笔记，用结构化信息恢复任务状态。
type Note struct {
	Goal    string            `json:"goal"`
	Facts   map[string]string `json:"facts"`
	Todo    []string          `json:"todo"`
	Updated string            `json:"updated"`
}

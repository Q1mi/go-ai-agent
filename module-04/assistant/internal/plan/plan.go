package plan

import "encoding/json"

// Task 表示 Plan-and-Execute 中的一项工具任务。
type Task struct {
	ID        string          `json:"id"`
	Tool      string          `json:"tool"`
	Args      json.RawMessage `json:"args"`
	DependsOn []string        `json:"depends_on"`
}

// Plan 表示一组带依赖关系的任务。
type Plan struct {
	Tasks []Task `json:"tasks"`
}

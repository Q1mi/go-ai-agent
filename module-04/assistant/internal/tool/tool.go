package tool

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/q1mi/assistant/internal/llm"
)

// Tool 是 Agent 可调用工具的统一接口。
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Call(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry 对应课件 4.2 的工具注册表。
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建工具注册表，并按工具名建立索引。
func NewRegistry(tools ...Tool) *Registry {
	registry := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, item := range tools {
		if item == nil {
			continue
		}
		registry.tools[item.Name()] = item
	}
	return registry
}

// Get 根据工具名查找工具。
func (registry *Registry) Get(name string) (Tool, bool) {
	if registry == nil {
		return nil, false
	}
	item, ok := registry.tools[name]
	return item, ok
}

// All 返回按名称排序的工具列表，保证输出稳定。
func (registry *Registry) All() []Tool {
	if registry == nil {
		return nil
	}
	out := make([]Tool, 0, len(registry.tools))
	for _, item := range registry.tools {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// ToolDefs 对应课件 4.5 中 Agent.toolDefs 的底层转换。
// Agent 保留 toolDefs 方法，方便和课件片段直接对应。
func (registry *Registry) ToolDefs() []llm.ToolDef {
	tools := registry.All()
	defs := make([]llm.ToolDef, 0, len(tools))
	for _, item := range tools {
		defs = append(defs, llm.ToolDef{
			Name:        item.Name(),
			Description: item.Description(),
			Parameters:  item.Parameters(),
		})
	}
	return defs
}

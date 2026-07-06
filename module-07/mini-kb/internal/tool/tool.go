package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/q1mi/minikb/internal/llm"
	"github.com/q1mi/minikb/internal/schema"
)

// Tool 是 Agent 可调用工具的统一接口。
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Call(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry 是工具注册表。
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建工具注册表。
func NewRegistry(tools ...Tool) *Registry {
	registry := &Registry{tools: map[string]Tool{}}
	for _, item := range tools {
		if item != nil {
			registry.tools[item.Name()] = item
		}
	}
	return registry
}

// Get 查询工具。
func (registry *Registry) Get(name string) (Tool, bool) {
	if registry == nil {
		return nil, false
	}
	item, ok := registry.tools[name]
	return item, ok
}

// All 返回按名称排序的工具。
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

// ToolDefs 转成模型工具声明。
func (registry *Registry) ToolDefs() []llm.ToolDef {
	items := registry.All()
	out := make([]llm.ToolDef, 0, len(items))
	for _, item := range items {
		out = append(out, llm.ToolDef{
			Name:        item.Name(),
			Description: item.Description(),
			Parameters:  item.Parameters(),
		})
	}
	return out
}

// TypedTool 把类型化 Go 函数包装成 Tool。
type TypedTool[T any] struct {
	name string
	desc string
	fn   func(ctx context.Context, args T) (string, error)
}

// NewTypedTool 创建类型安全工具。
func NewTypedTool[T any](name, desc string, fn func(ctx context.Context, args T) (string, error)) *TypedTool[T] {
	return &TypedTool[T]{name: name, desc: desc, fn: fn}
}

// Name 返回工具名。
func (tool *TypedTool[T]) Name() string { return tool.name }

// Description 返回工具描述。
func (tool *TypedTool[T]) Description() string { return tool.desc }

// Parameters 返回参数 JSON Schema。
func (tool *TypedTool[T]) Parameters() json.RawMessage {
	var zero T
	s, err := schema.Generate(zero)
	if err != nil {
		panic(err)
	}
	return schema.MustJSON(s)
}

// Call 解析 JSON 参数并调用函数。
func (tool *TypedTool[T]) Call(ctx context.Context, raw json.RawMessage) (string, error) {
	var args T
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("工具 %q 参数解析失败: %w", tool.name, err)
		}
	}
	return tool.fn(ctx, args)
}

package tool

import (
	"context"
	"fmt"
	"sort"
)

// Tool 是 Agent 可调用工具。
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, args string) (string, error)
}

// SimpleTool 把函数包装为工具。
type SimpleTool struct {
	name        string
	description string
	fn          func(ctx context.Context, args string) (string, error)
}

// New 创建工具。
func New(name, description string, fn func(ctx context.Context, args string) (string, error)) SimpleTool {
	return SimpleTool{name: name, description: description, fn: fn}
}

// Name 返回工具名。
func (tool SimpleTool) Name() string { return tool.name }

// Description 返回工具描述。
func (tool SimpleTool) Description() string { return tool.description }

// Call 执行工具。
func (tool SimpleTool) Call(ctx context.Context, args string) (string, error) {
	if tool.fn == nil {
		return "", fmt.Errorf("tool %s has nil handler", tool.name)
	}
	return tool.fn(ctx, args)
}

// Registry 保存工具。
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

// Names 返回工具名。
func (registry *Registry) Names() []string {
	if registry == nil {
		return nil
	}
	names := make([]string, 0, len(registry.tools))
	for name := range registry.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultRegistry 创建课堂演示工具集。
func DefaultRegistry() *Registry {
	return NewRegistry(
		New("get_weather", "查询城市天气", func(ctx context.Context, args string) (string, error) {
			return "北京：小雨，18℃，东北风 2 级。出行建议：带伞。", nil
		}),
		New("search_kb", "查询知识库政策", func(ctx context.Context, args string) (string, error) {
			return "知识库命中：退款政策为订单支付后 7天内可申请退款，需要提供订单号。", nil
		}),
	)
}

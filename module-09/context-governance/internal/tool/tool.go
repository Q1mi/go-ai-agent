package tool

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Tool 是简易 Agent 可调用工具的统一接口。
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

// SimpleTool 把普通函数包装成工具。
type SimpleTool struct {
	name        string
	description string
	fn          func(ctx context.Context, input string) (string, error)
}

// New 创建一个工具。
func New(name, description string, fn func(ctx context.Context, input string) (string, error)) SimpleTool {
	return SimpleTool{name: name, description: description, fn: fn}
}

// Name 返回工具名。
func (tool SimpleTool) Name() string { return tool.name }

// Description 返回工具描述。
func (tool SimpleTool) Description() string { return tool.description }

// Call 执行工具。
func (tool SimpleTool) Call(ctx context.Context, input string) (string, error) {
	if tool.fn == nil {
		return "", fmt.Errorf("工具 %s 没有处理函数", tool.name)
	}
	return tool.fn(ctx, input)
}

// Definitions 返回工具定义文本，模拟模型请求中的工具 schema 占用。
func Definitions(tools []Tool) string {
	items := make([]Tool, len(tools))
	copy(items, tools)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })

	var sb strings.Builder
	for _, item := range items {
		fmt.Fprintf(&sb, "- %s: %s\n", item.Name(), item.Description())
	}
	return sb.String()
}

// Names 返回工具名列表。
func Names(tools []Tool) []string {
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name())
	}
	sort.Strings(names)
	return names
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/q1mi/mcptools/internal/tool"
)

// BridgeAll 把 MCP Server 中的所有工具桥接为本项目 Tool。
func BridgeAll(ctx context.Context, client *StdioClient) ([]tool.Tool, error) {
	items, err := client.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tool.Tool, 0, len(items))
	for _, item := range items {
		out = append(out, bridgedTool{client: client, def: item})
	}
	return out, nil
}

type bridgedTool struct {
	client *StdioClient
	def    MCPTool
}

func (tool bridgedTool) Name() string { return tool.def.Name }

func (tool bridgedTool) Description() string { return tool.def.Description }

func (tool bridgedTool) Parameters() json.RawMessage {
	return defaultSchema(tool.def.InputSchema)
}

func (tool bridgedTool) Call(ctx context.Context, args json.RawMessage) (string, error) {
	result, err := tool.client.CallTool(ctx, tool.Name(), args)
	if err != nil {
		return "", err
	}
	text := result.Text()
	if result.IsError {
		return text, fmt.Errorf("MCP 工具 %q 返回错误", tool.Name())
	}
	return text, nil
}

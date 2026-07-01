package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/q1mi/mcptools/internal/tool"
)

// Server 是最小 MCP stdio Server，暴露 tool.Registry 中的工具。
type Server struct {
	registry *tool.Registry
	info     ServerInfo
}

// NewServer 创建 MCP Server。
func NewServer(registry *tool.Registry) *Server {
	if registry == nil {
		registry = tool.NewRegistry()
	}
	return &Server{
		registry: registry,
		info:     ServerInfo{Name: "go-mcp-tools", Version: "0.1.0"},
	}
}

// Serve 按 JSON-RPC over stdio 处理请求。每行一条 JSON 消息。
func (server *Server) Serve(ctx context.Context, reader io.Reader, writer io.Writer) error {
	buf := bufio.NewReader(reader)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := buf.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			server.handleLine(writer, line)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (server *Server) handleLine(writer io.Writer, line []byte) {
	var request rpcRequest
	if err := json.Unmarshal(line, &request); err != nil {
		writeRPCError(writer, nullID, errParseError, "解析 JSON-RPC 请求失败")
		return
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		if request.hasID() {
			writeRPCError(writer, request.ID, errInvalidRequest, "无效 JSON-RPC 请求")
		}
		return
	}
	if !request.hasID() {
		return
	}

	switch request.Method {
	case "initialize":
		server.handleInitialize(writer, request)
	case "tools/list":
		server.handleToolsList(writer, request)
	case "tools/call":
		server.handleToolsCall(writer, request)
	default:
		writeRPCError(writer, request.ID, errMethodNotFound, fmt.Sprintf("未知方法 %q", request.Method))
	}
}

func (server *Server) handleInitialize(writer io.Writer, request rpcRequest) {
	result := struct {
		ProtocolVersion string     `json:"protocolVersion"`
		Capabilities    any        `json:"capabilities"`
		ServerInfo      ServerInfo `json:"serverInfo"`
	}{
		ProtocolVersion: ProtocolVersion,
		Capabilities: map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		ServerInfo: server.info,
	}
	writeRPCResult(writer, request.ID, result)
}

func (server *Server) handleToolsList(writer io.Writer, request rpcRequest) {
	items := server.registry.All()
	tools := make([]MCPTool, 0, len(items))
	for _, item := range items {
		tools = append(tools, MCPTool{
			Name:        item.Name(),
			Description: item.Description(),
			InputSchema: defaultSchema(item.Parameters()),
		})
	}
	result := struct {
		Tools []MCPTool `json:"tools"`
	}{Tools: tools}
	writeRPCResult(writer, request.ID, result)
}

func (server *Server) handleToolsCall(writer io.Writer, request rpcRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if len(request.Params) == 0 || json.Unmarshal(request.Params, &params) != nil {
		writeRPCError(writer, request.ID, errInvalidParams, "tools/call 参数无效")
		return
	}
	if strings.TrimSpace(params.Name) == "" {
		writeRPCError(writer, request.ID, errInvalidParams, "tools/call 缺少工具名")
		return
	}
	if len(params.Arguments) == 0 {
		params.Arguments = json.RawMessage(`{}`)
	}
	item, ok := server.registry.Get(params.Name)
	if !ok {
		writeRPCResult(writer, request.ID, CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("工具 %q 不存在", params.Name)}},
			IsError: true,
		})
		return
	}
	output, err := item.Call(context.Background(), params.Arguments)
	if err != nil {
		text := strings.TrimSpace(output)
		if text != "" {
			text += "\n"
		}
		text += err.Error()
		writeRPCResult(writer, request.ID, CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: text}},
			IsError: true,
		})
		return
	}
	writeRPCResult(writer, request.ID, CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: output}},
	})
}

func writeRPCResult(writer io.Writer, id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		writeRPCError(writer, id, errInternalError, "序列化响应失败")
		return
	}
	_ = writeJSONLine(writer, rpcResponse{JSONRPC: "2.0", ID: id, Result: raw})
}

func writeRPCError(writer io.Writer, id json.RawMessage, code int, message string) {
	_ = writeJSONLine(writer, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func writeJSONLine(writer io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = writer.Write(raw)
	return err
}

func defaultSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	}
	return raw
}

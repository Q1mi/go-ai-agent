package mcp

import "encoding/json"

// ProtocolVersion 是本练习使用的 MCP 协议版本标识。
const ProtocolVersion = "2025-11-25"

// ServerInfo 描述 MCP Server 的基本信息。
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientInfo 描述 MCP Client 的基本信息。
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPTool 是 tools/list 返回给客户端的工具声明。
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ContentBlock 是 tools/call 返回的内容块。
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallToolResult 是 tools/call 的成功响应结构。
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// Text 合并 text 内容块，供 Agent 回填模型上下文。
func (result CallToolResult) Text() string {
	out := ""
	for _, block := range result.Content {
		if block.Type != "text" {
			continue
		}
		if out != "" {
			out += "\n"
		}
		out += block.Text
	}
	return out
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (request rpcRequest) hasID() bool {
	return len(request.ID) > 0 && string(request.ID) != "null"
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternalError  = -32603
)

var nullID = json.RawMessage("null")

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// StdioClient 通过子进程 stdin/stdout 调用 MCP Server。
type StdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu     sync.Mutex
	nextID int64
}

// NewStdioClient 启动 MCP Server 子进程。
func NewStdioClient(command string, args ...string) (*StdioClient, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("server command 不能为空")
	}
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	return newStdioClientFromCmd(cmd)
}

func newStdioClientFromCmd(cmd *exec.Cmd) (*StdioClient, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("打开 server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("打开 server stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 MCP server: %w", err)
	}
	return &StdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		nextID: 1,
	}, nil
}

// Close 关闭客户端并等待子进程退出。
func (client *StdioClient) Close() error {
	if client == nil {
		return nil
	}
	if client.stdin != nil {
		_ = client.stdin.Close()
	}
	if client.cmd == nil {
		return nil
	}
	return client.cmd.Wait()
}

// Initialize 完成 MCP 初始化握手。
func (client *StdioClient) Initialize(ctx context.Context) (ServerInfo, error) {
	var result struct {
		ProtocolVersion string     `json:"protocolVersion"`
		ServerInfo      ServerInfo `json:"serverInfo"`
	}
	params := struct {
		ProtocolVersion string     `json:"protocolVersion"`
		Capabilities    any        `json:"capabilities"`
		ClientInfo      ClientInfo `json:"clientInfo"`
	}{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo:      ClientInfo{Name: "go-mcp-client", Version: "0.1.0"},
	}
	if err := client.call(ctx, "initialize", params, &result); err != nil {
		return ServerInfo{}, err
	}
	if result.ProtocolVersion == "" {
		return ServerInfo{}, fmt.Errorf("initialize 响应缺少 protocolVersion")
	}
	return result.ServerInfo, nil
}

// Initialized 发送初始化完成通知。
func (client *StdioClient) Initialized(ctx context.Context) error {
	return client.notify(ctx, "notifications/initialized", map[string]any{})
}

// ListTools 读取 MCP Server 暴露的工具声明。
func (client *StdioClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := client.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool 调用 MCP 工具。
func (client *StdioClient) CallTool(ctx context.Context, name string, args json.RawMessage) (CallToolResult, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	params := struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}{Name: name, Arguments: args}
	var result CallToolResult
	if err := client.call(ctx, "tools/call", params, &result); err != nil {
		return CallToolResult{}, err
	}
	return result, nil
}

func (client *StdioClient) call(ctx context.Context, method string, params any, out any) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	id := client.nextID
	client.nextID++
	request, err := newRequest(id, method, params)
	if err != nil {
		return err
	}
	if err := client.write(ctx, request); err != nil {
		return err
	}
	line, err := readLine(ctx, client.stdout)
	if err != nil {
		return err
	}
	var response rpcResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return fmt.Errorf("解析 JSON-RPC 响应: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("MCP %s: %s", method, response.Error.Message)
	}
	if out == nil || len(response.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(response.Result, out); err != nil {
		return fmt.Errorf("解析 %s 响应结果: %w", method, err)
	}
	return nil
}

func (client *StdioClient) notify(ctx context.Context, method string, params any) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	request, err := newNotification(method, params)
	if err != nil {
		return err
	}
	return client.write(ctx, request)
}

func (client *StdioClient) write(ctx context.Context, request any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化 JSON-RPC 请求: %w", err)
	}
	raw = append(raw, '\n')
	if _, err := client.stdin.Write(raw); err != nil {
		return fmt.Errorf("写入 MCP server stdin: %w", err)
	}
	return nil
}

func newRequest(id int64, method string, params any) (rpcRequest, error) {
	rawID, _ := json.Marshal(id)
	rawParams, err := marshalParams(params)
	if err != nil {
		return rpcRequest{}, err
	}
	return rpcRequest{
		JSONRPC: "2.0",
		ID:      rawID,
		Method:  method,
		Params:  rawParams,
	}, nil
}

func newNotification(method string, params any) (rpcRequest, error) {
	rawParams, err := marshalParams(params)
	if err != nil {
		return rpcRequest{}, err
	}
	return rpcRequest{JSONRPC: "2.0", Method: method, Params: rawParams}, nil
}

func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化 JSON-RPC params: %w", err)
	}
	return raw, nil
}

func readLine(ctx context.Context, reader *bufio.Reader) ([]byte, error) {
	type lineResult struct {
		line []byte
		err  error
	}
	ch := make(chan lineResult, 1)
	go func() {
		line, err := reader.ReadBytes('\n')
		ch <- lineResult{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ch:
		if result.err != nil {
			return nil, fmt.Errorf("读取 MCP server stdout: %w", result.err)
		}
		return result.line, nil
	}
}

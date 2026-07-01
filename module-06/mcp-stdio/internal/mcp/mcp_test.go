package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/q1mi/mcptools/internal/builtin"
	"github.com/q1mi/mcptools/internal/tool"
)

func TestServerHandlesInitializeListAndCall(t *testing.T) {
	registry := tool.NewRegistry(
		builtin.NewCalcTool(),
		builtin.NewTimeTool(func() time.Time {
			return time.Date(2026, 6, 29, 10, 30, 0, 0, time.UTC)
		}),
	)
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"calc","arguments":{"expr":"1+2*3"}}}`,
		"",
	}, "\n")
	var output bytes.Buffer
	if err := NewServer(registry).Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	lines := nonEmptyLines(output.String())
	if len(lines) != 3 {
		t.Fatalf("response lines = %d, want 3: %s", len(lines), output.String())
	}

	var listResponse rpcResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResponse); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	var list struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(listResponse.Result, &list); err != nil {
		t.Fatalf("unmarshal list result: %v", err)
	}
	if len(list.Tools) != 2 {
		t.Fatalf("tools len = %d, want 2", len(list.Tools))
	}
	if string(list.Tools[0].InputSchema) == "" {
		t.Fatalf("inputSchema should be present")
	}

	var callResponse rpcResponse
	if err := json.Unmarshal([]byte(lines[2]), &callResponse); err != nil {
		t.Fatalf("unmarshal call response: %v", err)
	}
	var result CallToolResult
	if err := json.Unmarshal(callResponse.Result, &result); err != nil {
		t.Fatalf("unmarshal call result: %v", err)
	}
	if result.IsError {
		t.Fatalf("result should be success: %+v", result)
	}
	if got := result.Text(); got != "1+2*3 = 7" {
		t.Fatalf("result text = %q", got)
	}
}

func TestClientBridgeCallsServerTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	server := NewServer(tool.NewRegistry(builtin.NewCalcTool()))
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, serverReader, serverWriter)
	}()
	defer serverWriter.Close()

	client := &StdioClient{
		stdin:  clientWriter,
		stdout: bufio.NewReader(clientReader),
		nextID: 1,
	}
	defer client.Close()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := client.Initialized(ctx); err != nil {
		t.Fatalf("Initialized() error = %v", err)
	}
	tools, err := BridgeAll(ctx, client)
	if err != nil {
		t.Fatalf("BridgeAll() error = %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(tools))
	}
	if string(tools[0].Parameters()) == "" {
		t.Fatalf("bridged tool should expose parameters")
	}
	text, err := tools[0].Call(ctx, json.RawMessage(`{"expr":"(8+4)/3"}`))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if text != "(8+4)/3 = 4" {
		t.Fatalf("Call() = %q", text)
	}

	_ = clientWriter.Close()
	if err := <-done; err != nil && ctx.Err() == nil {
		t.Fatalf("server returned error: %v", err)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

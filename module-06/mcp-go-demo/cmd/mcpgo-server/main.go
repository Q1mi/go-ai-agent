package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/q1mi/mcpgoinspector/internal/calc"
)

const (
	defaultAddr = "0.0.0.0:8887"
)

func main() {
	s := newServer()

	addr := os.Getenv("MCP_HTTP_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	httpServer := server.NewStreamableHTTPServer(
		s,
		server.WithEndpointPath("/mcp"),
	)

	go func() {
		fmt.Fprintf(os.Stderr, "MCP HTTP listening on http://%s/mcp\n", addr)
		if err := httpServer.Start(addr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newServer() *server.MCPServer {
	s := server.NewMCPServer(
		"M06 mcp-go demo",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	s.AddTool(newGetTimeTool(), handleGetTime)
	s.AddTool(newCalcTool(), handleCalc)
	return s
}

func newGetTimeTool() mcp.Tool {
	return mcp.NewTool("get_time",
		mcp.WithDescription("返回当前时间，支持按 IANA 时区格式化"),
		mcp.WithString("timezone",
			mcp.Description("IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"),
		),
	)
}

func handleGetTime(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	timezone := request.GetString("timezone", "")
	loc := time.Local
	if timezone != "" {
		loaded, err := time.LoadLocation(timezone)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("无效时区 %q: %v", timezone, err)), nil
		}
		loc = loaded
	}
	return mcp.NewToolResultText(time.Now().In(loc).Format(time.RFC3339)), nil
}

func newCalcTool() mcp.Tool {
	return mcp.NewTool("calc",
		mcp.WithDescription("计算只包含数字、括号、+、-、*、/ 的算术表达式"),
		mcp.WithString("expr",
			mcp.Required(),
			mcp.Description("四则运算表达式，例如 1+2*3"),
		),
	)
}

func handleCalc(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	expr, err := request.RequireString("expr")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	expr = calc.Normalize(expr)
	if expr == "" {
		return mcp.NewToolResultError("expr 不能为空"), nil
	}
	value, err := calc.Eval(expr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("%s = %s", expr, calc.Format(value))), nil
}

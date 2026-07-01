package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/q1mi/mcptools/internal/mcp"
)

func main() {
	serverCommand := flag.String("server", "go", "MCP server 命令")
	serverArgs := flag.String("server-args", "run,./cmd/mcpserver", "MCP server 参数，使用英文逗号分隔")
	listOnly := flag.Bool("list", false, "只列出工具")
	toolName := flag.String("tool", "get_time", "要调用的工具名")
	rawArgs := flag.String("args", `{}`, "工具参数 JSON")
	timeout := flag.Duration("timeout", 30*time.Second, "调用超时时间")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := mcp.NewStdioClient(*serverCommand, splitArgs(*serverArgs)...)
	if err != nil {
		fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	if _, err := client.Initialize(ctx); err != nil {
		fatal(err)
	}
	if err := client.Initialized(ctx); err != nil {
		fatal(err)
	}
	if *listOnly {
		tools, err := client.ListTools(ctx)
		if err != nil {
			fatal(err)
		}
		printJSON(tools)
		return
	}

	args := json.RawMessage(strings.TrimSpace(*rawArgs))
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if !json.Valid(args) {
		fatal(fmt.Errorf("-args 必须是合法 JSON"))
	}
	result, err := client.CallTool(ctx, *toolName, args)
	if err != nil {
		fatal(err)
	}
	fmt.Println(result.Text())
	if result.IsError {
		os.Exit(1)
	}
}

func splitArgs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "[错误]", err)
	os.Exit(1)
}

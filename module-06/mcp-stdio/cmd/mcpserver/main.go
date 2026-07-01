package main

import (
	"context"
	"fmt"
	"os"

	"github.com/q1mi/mcptools/internal/builtin"
	"github.com/q1mi/mcptools/internal/mcp"
	"github.com/q1mi/mcptools/internal/tool"
)

func main() {
	registry := tool.NewRegistry(
		builtin.NewTimeTool(nil),
		builtin.NewCalcTool(),
	)
	if err := mcp.NewServer(registry).Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleCalc(t *testing.T) {
	result, err := handleCalc(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "calc",
			Arguments: map[string]any{"expr": "12*(3+4)"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("expected success result: %+v", result)
	}
	if got := resultText(t, result); got != "12*(3+4) = 84" {
		t.Fatalf("text = %q", got)
	}
}

func TestHandleGetTimeInvalidTimezone(t *testing.T) {
	result, err := handleGetTime(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "get_time",
			Arguments: map[string]any{"timezone": "Mars/Base"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("expected error result: %+v", result)
	}
	if !strings.Contains(resultText(t, result), "无效时区") {
		t.Fatalf("unexpected text = %q", resultText(t, result))
	}
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("empty result: %+v", result)
	}
	text, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("content is not text: %#v", result.Content[0])
	}
	return text.Text
}

package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/q1mi/mcptools/internal/llm"
)

func TestProviderChatWithTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "calc" {
			t.Fatalf("tools = %+v", req.Tools)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"tool_calls": []map[string]any{{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "calc",
							"arguments": "{\"expr\":\"1+2\"}",
						},
					}},
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := New(Config{
		BaseURL:      server.URL,
		APIKey:       "key",
		DefaultModel: "test",
		ToolCalling:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := provider.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "1+2"}},
		Tools: []llm.ToolDef{{
			Name:       "calc",
			Parameters: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 || string(resp.ToolCalls[0].Args) != `{"expr":"1+2"}` {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
}

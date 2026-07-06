package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/q1mi/traceagent/internal/llm"
	"github.com/q1mi/traceagent/internal/obs"
	"github.com/q1mi/traceagent/internal/tool"
)

func TestAgentRunCreatesTrace(t *testing.T) {
	tracing, err := obs.SetupMemoryTracing("test-traceagent")
	if err != nil {
		t.Fatal(err)
	}
	defer tracing.Shutdown(context.Background())

	server := newTestModelServer(t)
	defer server.Close()
	provider, err := llm.NewOpenAICompatibleProvider(llm.OpenAICompatibleConfig{
		Name:         "test-deepseek",
		BaseURL:      server.URL,
		APIKey:       "test-key",
		DefaultModel: "test-model",
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	bot := &Agent{
		Provider: provider,
		Model:    provider.DefaultModel(),
		Tools:    tool.DefaultRegistry(),
	}
	result, err := bot.Run(context.Background(), "北京今天需要带伞吗？")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Answer, "带伞") {
		t.Fatalf("answer=%q", result.Answer)
	}
	tree := tracing.Exporter.Tree(result.TraceID)
	for _, want := range []string{"invoke_agent kbot", "chat test-model", "execute_tool get_weather"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("trace tree missing %q:\n%s", want, tree)
		}
	}
}

func newTestModelServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req struct {
			Model    string        `json:"model"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		allText := llm.FormatMessages(req.Messages)
		for i, msg := range req.Messages {
			if msg.Role == llm.RoleTool {
				t.Errorf("request message %d should not use role=tool", i)
			}
		}
		content := "北京今天小雨，建议带伞。依据 get_weather 工具返回的天气信息。"
		if strings.Contains(allText, "选择一个工具") || strings.Contains(allText, "\"tool\"") {
			content = `{"tool":"get_weather","args":"北京","reason":"问题询问天气和是否带伞"}`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-test",
			"model": req.Model,
			"choices": []map[string]any{
				{
					"message": map[string]string{"role": "assistant", "content": content},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     12,
				"completion_tokens": 8,
				"total_tokens":      20,
			},
		})
	}))
}

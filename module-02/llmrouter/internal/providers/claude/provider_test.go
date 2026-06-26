package claude

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/q1mi/llmrouter/internal/llm"
)

func TestAdaptRequest(t *testing.T) {
	provider, err := New(Config{
		BaseURL:      "https://api.example.com/v1",
		APIKey:       "key",
		DefaultModel: "claude-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := provider.adaptRequest(llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "规则一"},
			{Role: llm.RoleSystem, Content: "规则二"},
			{Role: llm.RoleUser, Content: "你好"},
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if request.System != "规则一\n\n规则二" {
		t.Fatalf("System = %q", request.System)
	}
	if request.Model != "claude-test" || request.MaxTokens != 1024 {
		t.Fatalf("request = %+v", request)
	}
}

func TestAdaptRequestRejectsToolRole(t *testing.T) {
	provider, err := New(Config{
		BaseURL:      "https://api.example.com/v1",
		APIKey:       "key",
		DefaultModel: "claude-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.adaptRequest(llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleTool, Content: "result"}},
	}, false)
	if err == nil {
		t.Fatal("期望不支持 tool role 错误")
	}
}

func TestChat(t *testing.T) {
	var request anthropicRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != defaultAnthropicVersion {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode: %v", err)
		}
		_, _ = w.Write([]byte(
			`{"model":"claude-test","content":[{"type":"thinking","thinking":"..."},` +
				`{"type":"text","text":"你"},{"type":"text","text":"好"}],` +
				`"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2}}`,
		))
	}))
	defer server.Close()

	provider, err := New(Config{
		BaseURL:      server.URL + "/v1",
		APIKey:       "key",
		DefaultModel: "claude-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "你好" || response.Usage.TotalTokens() != 6 {
		t.Fatalf("response = %+v", response)
	}
}

func TestChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"event: message_start\n" +
				"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4}}}\n\n",
		))
		_, _ = w.Write([]byte(
			"event: content_block_delta\n" +
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"你\"}}\n\n",
		))
		_, _ = w.Write([]byte(
			"event: message_delta\n" +
				"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":2}}\n\n",
		))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	provider, err := New(Config{
		BaseURL:      server.URL,
		APIKey:       "key",
		DefaultModel: "claude-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := provider.ChatStream(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var content strings.Builder
	var usage llm.Usage
	var done bool
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Content)
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
		done = done || chunk.Done
	}
	if content.String() != "你" || usage.TotalTokens() != 6 || !done {
		t.Fatalf("content=%q usage=%+v done=%v", content.String(), usage, done)
	}
}

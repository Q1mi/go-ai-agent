package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/q1mi/llmrouter/internal/llm"
)

func TestChatUsesDefaultModel(t *testing.T) {
	var request chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"model":"test-model","choices":[{"message":{"content":"hello"},"finish_reason":"stop"}],` +
				`"usage":{"prompt_tokens":3,"completion_tokens":2}}`,
		))
	}))
	defer server.Close()

	provider, err := New(Config{
		Name:         "test",
		BaseURL:      server.URL,
		APIKey:       "test-key",
		DefaultModel: "test-model",
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
	if request.Model != "test-model" {
		t.Fatalf("Model = %q", request.Model)
	}
	if response.Content != "hello" || response.Usage.TotalTokens() != 5 {
		t.Fatalf("response = %+v", response)
	}
}

func TestChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request chatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode: %v", err)
		}
		if request.StreamOptions == nil || !request.StreamOptions.IncludeUsage {
			t.Errorf("stream_options = %+v", request.StreamOptions)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n"))
		_, _ = w.Write([]byte(
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\n",
		))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider, err := New(Config{
		Name:         "test",
		BaseURL:      server.URL,
		DefaultModel: "test-model",
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
	if content.String() != "你好" || usage.TotalTokens() != 5 || !done {
		t.Fatalf("content=%q usage=%+v done=%v", content.String(), usage, done)
	}
}

func TestParseStreamEventRejectsInvalidJSON(t *testing.T) {
	if _, _, err := parseStreamEvent([]byte(`{"choices":`)); err == nil {
		t.Fatal("期望 JSON 解析错误")
	}
}

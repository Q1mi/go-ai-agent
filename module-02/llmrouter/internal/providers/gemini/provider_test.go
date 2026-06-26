package gemini

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
		APIKey:       "key",
		DefaultModel: "gemini-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	temperature := 0.2
	maxTokens := 256
	request, model, err := provider.adaptRequest(llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "规则一"},
			{Role: llm.RoleSystem, Content: "规则二"},
			{Role: llm.RoleUser, Content: "你好"},
			{Role: llm.RoleAssistant, Content: "你好，我能帮你什么？"},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model != "gemini-test" {
		t.Fatalf("model = %q", model)
	}
	if request.SystemInstruction == nil || len(request.SystemInstruction.Parts) != 2 {
		t.Fatalf("SystemInstruction = %+v", request.SystemInstruction)
	}
	if len(request.Contents) != 2 ||
		request.Contents[0].Role != "user" ||
		request.Contents[1].Role != "model" {
		t.Fatalf("Contents = %+v", request.Contents)
	}
	if request.GenerationConfig == nil ||
		request.GenerationConfig.Temperature == nil ||
		*request.GenerationConfig.Temperature != temperature ||
		request.GenerationConfig.MaxOutputTokens == nil ||
		*request.GenerationConfig.MaxOutputTokens != maxTokens {
		t.Fatalf("GenerationConfig = %+v", request.GenerationConfig)
	}
}

func TestAdaptRequestRejectsToolRole(t *testing.T) {
	provider, err := New(Config{
		APIKey:       "key",
		DefaultModel: "gemini-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = provider.adaptRequest(llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleTool, Content: "result"}},
	})
	if err == nil {
		t.Fatal("期望不支持 tool role 错误")
	}
}

func TestChat(t *testing.T) {
	var request generateContentRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "key" {
			t.Errorf("x-goog-api-key = %q", r.Header.Get("x-goog-api-key"))
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"candidates":[{"content":{"parts":[{"text":"你"},{"text":"好"}],"role":"model"},` +
				`"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,` +
				`"candidatesTokenCount":2,"totalTokenCount":6},"modelVersion":"gemini-test"}`,
		))
	}))
	defer server.Close()

	provider, err := New(Config{
		BaseURL:      server.URL + "/v1beta",
		APIKey:       "key",
		DefaultModel: "gemini-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "你是助手"},
			{Role: llm.RoleUser, Content: "hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.SystemInstruction == nil || request.SystemInstruction.Parts[0].Text != "你是助手" {
		t.Fatalf("SystemInstruction = %+v", request.SystemInstruction)
	}
	if response.Content != "你好" || response.Usage.TotalTokens() != 6 {
		t.Fatalf("response = %+v", response)
	}
}

func TestChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:streamGenerateContent" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("alt") != "sse" {
			t.Errorf("alt = %q", r.URL.Query().Get("alt"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"你\"}],\"role\":\"model\"}}]," +
				"\"usageMetadata\":{\"promptTokenCount\":4}}\n\n",
		))
		_, _ = w.Write([]byte(
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"好\"}],\"role\":\"model\"}," +
				"\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4," +
				"\"candidatesTokenCount\":2,\"totalTokenCount\":6}}\n\n",
		))
	}))
	defer server.Close()

	provider, err := New(Config{
		BaseURL:      server.URL + "/v1beta",
		APIKey:       "key",
		DefaultModel: "gemini-test",
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
	if content.String() != "你好" || usage.TotalTokens() != 6 || !done {
		t.Fatalf("content=%q usage=%+v done=%v", content.String(), usage, done)
	}
}

func TestChatStreamTreatsEOFAsDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"你好\"}],\"role\":\"model\"}}]}\n\n",
		))
	}))
	defer server.Close()

	provider, err := New(Config{
		BaseURL:      server.URL + "/v1beta",
		APIKey:       "key",
		DefaultModel: "gemini-test",
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
	var done bool
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Content)
		done = done || chunk.Done
	}
	if content.String() != "你好" || !done {
		t.Fatalf("content=%q done=%v", content.String(), done)
	}
}

func TestParseStreamEventRejectsInvalidJSON(t *testing.T) {
	if _, _, err := parseStreamEvent([]byte(`{"candidates":`)); err == nil {
		t.Fatal("期望 JSON 解析错误")
	}
}

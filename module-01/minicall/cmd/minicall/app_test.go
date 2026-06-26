package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/q1mi/minicall/internal/llm"
	"github.com/q1mi/minicall/internal/transport"
)

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "https://api.example.com/v1/")
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "test-model")

	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "test-key" || cfg.Model != "test-model" {
		t.Fatalf("配置读取错误: %+v", cfg)
	}
}

func TestLoadConfigFromEnvReportsMissingValues(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	_, err := loadConfigFromEnv()
	if err == nil {
		t.Fatal("期望返回缺少配置错误")
	}
	for _, name := range []string{"LLM_BASE_URL", "LLM_API_KEY", "LLM_MODEL"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("错误 %q 未包含 %s", err, name)
		}
	}
}

func TestCallModel(t *testing.T) {
	var request llm.ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("解析请求体: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"choices":[{"message":{"role":"assistant","content":"Go 很适合。"}}],` +
				`"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13}}`,
		))
	}))
	defer server.Close()

	retry := transport.DefaultRetryConfig()
	retry.MaxRetries = 0
	got, err := callModel(
		context.Background(),
		transport.NewClient(transport.WithRetry(retry)),
		Config{
			BaseURL: server.URL + "/v1",
			APIKey:  "test-key",
			Model:   "test-model",
		},
		"为什么选择 Go？",
	)
	if err != nil {
		t.Fatal(err)
	}

	if request.Model != "test-model" || request.Stream {
		t.Fatalf("请求参数错误: %+v", request)
	}
	if len(request.Messages) != 1 || request.Messages[0].Content != "为什么选择 Go？" {
		t.Fatalf("Messages = %+v", request.Messages)
	}
	if got.Content != "Go 很适合。" {
		t.Fatalf("Content = %q", got.Content)
	}
	if got.InputTokens != 8 || got.OutputTokens != 5 || got.TotalTokens != 13 {
		t.Fatalf("token 统计错误: %+v", got)
	}
}

func TestCallModelReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"message":"invalid key"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := callModel(
		context.Background(),
		transport.NewClient(),
		Config{
			BaseURL: server.URL,
			APIKey:  "bad-key",
			Model:   "test-model",
		},
		"hello",
	)
	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("err = %v", err)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDryRun(t *testing.T) {
	docsDir := t.TempDir()
	writeDoc(t, docsDir, "timeout.md", "# 超时配置\n默认 timeout 是 30 秒。")

	var out bytes.Buffer
	err := run([]string{
		"-dry-run",
		"-product", "测试产品",
		"-docs", docsDir,
		"-question", "怎么设置超时？",
		"-current-time", "2026-06-26T11:00:00+08:00",
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"# 文档问答助手上下文方案",
		"测试产品",
		"怎么设置超时？",
		"Token 预算",
		"Prompt Caching 稳定前缀",
		"2026-06-26T11:00:00+08:00",
		"默认 timeout 是 30 秒",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("输出未包含 %q:\n%s", want, got)
		}
	}
}

func TestRunCallsModelWithRetrievedDocs(t *testing.T) {
	docsDir := t.TempDir()
	writeDoc(t, docsDir, "timeout.md", "# 超时配置\n默认 timeout 是 30 秒。")

	var request struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode: %v", err)
		}
		_, _ = w.Write([]byte(
			`{"model":"test-model","choices":[{"message":{"content":"把 timeout 设置为 30 秒。[D1]"},"finish_reason":"stop"}],` +
				`"usage":{"prompt_tokens":20,"completion_tokens":6,"total_tokens":26}}`,
		))
	}))
	defer server.Close()

	t.Setenv("LLM_BASE_URL", server.URL+"/v1")
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("LLM_MODEL", "test-model")

	var out bytes.Buffer
	err := run([]string{
		"-docs", docsDir,
		"-question", "怎么设置超时？",
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if request.Model != "test-model" {
		t.Fatalf("model = %q", request.Model)
	}
	if len(request.Messages) == 0 || !strings.Contains(request.Messages[0].Content, "默认 timeout 是 30 秒") {
		t.Fatalf("messages = %+v", request.Messages)
	}
	got := out.String()
	for _, want := range []string{
		"把 timeout 设置为 30 秒",
		"token: input=20 output=6 total=26",
		"retrieved:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("输出未包含 %q:\n%s", want, got)
		}
	}
}

func writeDoc(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

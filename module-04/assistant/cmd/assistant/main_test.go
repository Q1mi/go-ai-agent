package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSingleTurn(t *testing.T) {
	server := newChatServer(t, []string{`{
		"choices":[{"message":{"content":"普通回答来自模型。"}}],
		"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}
	}`})
	defer server.Close()
	setLLMEnv(t, server.URL)

	var stdout, stderr bytes.Buffer
	err := run(
		[]string{"你好"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	if !strings.Contains(got, "普通回答来自模型") || !strings.Contains(got, "[完成]") {
		t.Fatalf("stdout = %s stderr = %s", got, stderr.String())
	}
}

func TestRunReactMode(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var request struct {
			Tools []any `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Tools) > 0 {
			t.Fatalf("react mode should not send tools: %+v", request.Tools)
		}
		switch requestCount {
		case 1:
			_, _ = w.Write([]byte(`{
				"choices":[{"message":{"content":"Thought: 需要计算表达式。\nAction: calculator\nAction Input: {\"expr\":\"(1+2)*3\"}"}}],
				"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"choices":[{"message":{"content":"Thought: 工具结果已经足够。\nFinal Answer: 计算结果是 9。"}}],
				"usage":{"prompt_tokens":30,"completion_tokens":8,"total_tokens":38}
			}`))
		default:
			t.Fatalf("unexpected request %d", requestCount)
		}
	}))
	defer server.Close()
	setLLMEnv(t, server.URL)

	var stdout, stderr bytes.Buffer
	err := run(
		[]string{"-mode", "react", "计算 (1+2)*3"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	if !strings.Contains(got, "[调用工具] calculator") || !strings.Contains(got, "计算结果是 9") {
		t.Fatalf("stdout = %s stderr = %s", stdout.String(), stderr.String())
	}
}

func TestRunIdentityQuestion(t *testing.T) {
	server := newChatServer(t, []string{`{
		"choices":[{"message":{"content":"我是由真实大模型驱动的命令行 AI 助手。"}}],
		"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}
	}`})
	defer server.Close()
	setLLMEnv(t, server.URL)

	var stdout, stderr bytes.Buffer
	err := run(
		[]string{"你是谁"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	if !strings.Contains(got, "我是由真实大模型驱动的命令行 AI 助手") {
		t.Fatalf("stdout = %s stderr = %s", got, stderr.String())
	}
}

func TestRunUsesLLMProviderForPlainAnswer(t *testing.T) {
	var request struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"我是一个真实模型驱动的命令行助手。"}}],
			"usage":{"prompt_tokens":12,"completion_tokens":6,"total_tokens":18}
		}`))
	}))
	defer server.Close()
	t.Setenv("LLM_BASE_URL", server.URL+"/v1")
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("LLM_MODEL", "test-model")

	var stdout, stderr bytes.Buffer
	err := run(
		[]string{"你是谁"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.Model != "test-model" {
		t.Fatalf("model = %q", request.Model)
	}
	if len(request.Tools) == 0 {
		t.Fatalf("tools = %+v", request.Tools)
	}
	if !strings.Contains(stdout.String(), "我是一个真实模型驱动的命令行助手") {
		t.Fatalf("stdout = %s stderr = %s", stdout.String(), stderr.String())
	}
}

func TestApplyBackspace(t *testing.T) {
	tests := map[string]string{
		"abc\x7fd":     "abd",
		"abc\bd":       "abd",
		"你是谁\x7f\x7f好": "你好",
		"\x7fabc":      "abc",
	}
	for input, want := range tests {
		if got := applyBackspace(input); got != want {
			t.Fatalf("applyBackspace(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPromptLineReaderBufferedBackspace(t *testing.T) {
	var stdout bytes.Buffer
	reader := newPromptLineReader(strings.NewReader("你是谁\x7f\nexit\n"), &stdout)

	first, err := reader.ReadLine(context.Background(), "> ")
	if err != nil {
		t.Fatal(err)
	}
	if first != "你是" {
		t.Fatalf("first = %q", first)
	}
	second, err := reader.ReadLine(context.Background(), "> ")
	if err != nil {
		t.Fatal(err)
	}
	if second != "exit" {
		t.Fatalf("second = %q", second)
	}
	if got := stdout.String(); strings.Count(got, "> ") != 2 {
		t.Fatalf("stdout = %q", got)
	}
}

func newChatServer(t *testing.T, responses []string) *httptest.Server {
	t.Helper()
	requestCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if requestCount >= len(responses) {
			t.Fatalf("unexpected request %d", requestCount+1)
		}
		_, _ = w.Write([]byte(responses[requestCount]))
		requestCount++
	}))
}

func setLLMEnv(t *testing.T, serverURL string) {
	t.Helper()
	t.Setenv("LLM_BASE_URL", serverURL+"/v1")
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("LLM_MODEL", "test-model")
}

func TestRunUsesFunctionCallingWhenModelRequestsTool(t *testing.T) {
	requestCount := 0
	var secondRequest struct {
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			_, _ = w.Write([]byte(`{
				"choices":[{
					"message":{
						"content":"我先调用计算器。",
						"tool_calls":[{
							"id":"call_1",
							"type":"function",
							"function":{"name":"calculator","arguments":"{\"expr\":\"1+2*3\"}"}
						}]
					}
				}],
				"usage":{"prompt_tokens":20,"completion_tokens":8,"total_tokens":28}
			}`))
		case 2:
			if err := json.NewDecoder(r.Body).Decode(&secondRequest); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{
				"choices":[{"message":{"content":"计算结果是 7。"}}],
				"usage":{"prompt_tokens":30,"completion_tokens":6,"total_tokens":36}
			}`))
		default:
			t.Fatalf("unexpected request %d", requestCount)
		}
	}))
	defer server.Close()
	t.Setenv("LLM_BASE_URL", server.URL+"/v1")
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("LLM_MODEL", "test-model")

	var stdout, stderr bytes.Buffer
	err := run(
		[]string{"计算 1+2*3"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 {
		t.Fatalf("requestCount = %d", requestCount)
	}
	foundToolResult := false
	for _, message := range secondRequest.Messages {
		if message.Role == "tool" && strings.Contains(message.Content, "1+2*3 = 7") && message.ToolCallID == "call_1" {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("second messages = %+v", secondRequest.Messages)
	}
	got := stdout.String()
	if !strings.Contains(got, "[调用工具] calculator") ||
		!strings.Contains(got, "1+2*3 = 7") ||
		!strings.Contains(got, "计算结果是 7") {
		t.Fatalf("stdout = %s stderr = %s", got, stderr.String())
	}
}

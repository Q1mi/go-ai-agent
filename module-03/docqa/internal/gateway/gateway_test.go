package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/q1mi/docqa-context/internal/llm"
)

func TestGatewayChatOpenAICompatible(t *testing.T) {
	var got struct {
		Model    string        `json:"model"`
		Messages []llm.Message `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(
			`{"model":"test-model","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],` +
				`"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		))
	}))
	defer server.Close()

	modelGateway, err := New(Config{Providers: []ProviderConfig{{
		Name:         "test",
		BaseURL:      server.URL + "/v1",
		APIKey:       "test-key",
		DefaultModel: "test-model",
	}}})
	if err != nil {
		t.Fatal(err)
	}

	response, err := modelGateway.Chat(context.Background(), llm.NewChatRequest(
		"",
		[]llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	))
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "test-model" {
		t.Fatalf("model = %q", got.Model)
	}
	if response.Content != "ok" || response.Usage.TotalTokens() != 7 {
		t.Fatalf("response = %+v", response)
	}
}

func TestLoadFromEnvUsesGenericLLMConfig(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("LLM_BASE_URL", "https://example.com/v1")
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("LLM_MODEL", "model")

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Providers) != 1 {
		t.Fatalf("providers = %+v", config.Providers)
	}
	if config.Providers[0].Name != "llm" || config.Providers[0].DefaultModel != "model" {
		t.Fatalf("provider = %+v", config.Providers[0])
	}
}

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, prefix := range []string{"LLM", "DEEPSEEK", "OPENAI", "DOUBAO", "OLLAMA"} {
		t.Setenv(prefix+"_BASE_URL", "")
		t.Setenv(prefix+"_API_KEY", "")
		t.Setenv(prefix+"_MODEL", "")
	}
	t.Setenv("LLM_PROVIDER_NAME", "")
}

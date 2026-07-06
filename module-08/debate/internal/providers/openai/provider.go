package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/q1mi/debate/internal/llm"
)

// Config 描述 OpenAI 兼容 Chat Completions 配置。
type Config struct {
	Name         string
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// Provider 调用 OpenAI 兼容 /chat/completions。
type Provider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewFromEnv 从 LLM_* 环境变量创建 Provider。
func NewFromEnv() (*Provider, error) {
	return New(Config{
		Name:         envOrDefault("LLM_PROVIDER_NAME", "openai-compatible"),
		BaseURL:      envOrDefault("LLM_BASE_URL", "https://api.deepseek.com"),
		APIKey:       strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		DefaultModel: strings.TrimSpace(os.Getenv("LLM_MODEL")),
	})
}

// New 创建 Provider。
func New(config Config) (*Provider, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "openai-compatible"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("LLM_BASE_URL 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("无效 LLM_BASE_URL: %q", baseURL)
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" && !allowEmptyAPIKey(parsed.Host) {
		return nil, errors.New("LLM_API_KEY 不能为空")
	}
	model := strings.TrimSpace(config.DefaultModel)
	if model == "" {
		return nil, errors.New("LLM_MODEL 不能为空")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &Provider{name: name, baseURL: baseURL, apiKey: apiKey, defaultModel: model, httpClient: client}, nil
}

// Name 返回 Provider 名称。
func (provider *Provider) Name() string {
	return provider.name
}

// Chat 调用 /chat/completions。
func (provider *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = provider.defaultModel
	}
	wireReq := chatRequest{
		Model:    model,
		Messages: toMessages(req.Messages),
		Stream:   false,
	}
	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if provider.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}
	resp, err := provider.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用模型接口: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, parseAPIError(provider.name, resp)
	}
	return parseResponse(resp.Body)
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func toMessages(messages []llm.Message) []message {
	out := make([]message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, message{Role: string(msg.Role), Content: msg.Content})
	}
	return out
}

func parseResponse(body io.Reader) (*llm.ChatResponse, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage llm.Usage `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Choices) == 0 {
		return nil, errors.New("模型响应中没有 choices")
	}
	return &llm.ChatResponse{
		Content: strings.TrimSpace(payload.Choices[0].Message.Content),
		Usage:   payload.Usage,
	}, nil
}

func parseAPIError(providerName string, resp *http.Response) error {
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if readErr != nil {
		return fmt.Errorf("%s: HTTP %d，读取错误响应失败: %w", providerName, resp.StatusCode, readErr)
	}
	message := strings.TrimSpace(string(raw))
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error.Message != "" {
		message = payload.Error.Message
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", providerName, resp.StatusCode, message)
}

func allowEmptyAPIKey(host string) bool {
	host = strings.ToLower(host)
	return strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]")
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

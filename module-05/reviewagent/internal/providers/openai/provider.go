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

	"github.com/q1mi/reviewagent/internal/llm"
)

// Config 描述一个 OpenAI 兼容 Provider。
type Config struct {
	Name         string
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// Provider 是普通 Chat Completions 调用实现。
type Provider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewFromEnv 从环境变量创建 Provider。
//
// 必填：LLM_MODEL。远程模型通常还需要 LLM_API_KEY。
// LLM_BASE_URL 为空时默认使用 DeepSeek 的 OpenAI 兼容地址。
func NewFromEnv() (*Provider, error) {
	return New(Config{
		Name:         envOrDefault("LLM_PROVIDER_NAME", "openai-compatible"),
		BaseURL:      envOrDefault("LLM_BASE_URL", "https://api.deepseek.com"),
		APIKey:       strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		DefaultModel: strings.TrimSpace(os.Getenv("LLM_MODEL")),
	})
}

// New 校验配置并创建 Provider。
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
	model := strings.TrimSpace(config.DefaultModel)
	if model == "" {
		return nil, errors.New("LLM_MODEL 不能为空")
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" && !allowEmptyAPIKey(parsed.Host) {
		return nil, errors.New("LLM_API_KEY 不能为空")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &Provider{
		name:         name,
		baseURL:      baseURL,
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   client,
	}, nil
}

// Name 返回 Provider 名称。
func (provider *Provider) Name() string {
	return provider.name
}

// DefaultModel 返回默认模型。
func (provider *Provider) DefaultModel() string {
	return provider.defaultModel
}

// Capabilities 返回 Provider 能力。
func (provider *Provider) Capabilities() llm.Capability {
	return llm.Capability{}
}

// Chat 调用 OpenAI 兼容 /chat/completions 接口。
func (provider *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	model, err := llm.EffectiveModel(req.Model, provider.defaultModel)
	if err != nil {
		return nil, err
	}
	wireRequest := openAIChatRequest{
		Model:       model,
		Messages:    append([]llm.Message(nil), req.Messages...),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}
	body, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, fmt.Errorf("序列化模型请求: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建模型请求: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	if provider.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}

	response, err := provider.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("调用模型接口: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, parseAPIError(provider.name, response)
	}
	return parseChatResponse(response.Body, model)
}

// ChatStream 保留 Provider 接口中的流式入口。
func (provider *Provider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("%s: M05 练习暂未实现 ChatStream", provider.name)
}

type openAIChatRequest struct {
	Model       string        `json:"model"`
	Messages    []llm.Message `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

func parseChatResponse(body io.Reader, fallbackModel string) (*llm.ChatResponse, error) {
	var payload struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析模型响应: %w", err)
	}
	if len(payload.Choices) == 0 {
		return nil, errors.New("模型响应中没有 choices")
	}
	model := payload.Model
	if model == "" {
		model = fallbackModel
	}
	outputTokens := payload.Usage.CompletionTokens
	if outputTokens == 0 && payload.Usage.TotalTokens > payload.Usage.PromptTokens {
		outputTokens = payload.Usage.TotalTokens - payload.Usage.PromptTokens
	}
	return &llm.ChatResponse{
		Content:      strings.TrimSpace(payload.Choices[0].Message.Content),
		Model:        model,
		FinishReason: payload.Choices[0].FinishReason,
		Usage: llm.Usage{
			InputTokens:  payload.Usage.PromptTokens,
			OutputTokens: outputTokens,
		},
	}, nil
}

func parseAPIError(providerName string, response *http.Response) error {
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if readErr != nil {
		return fmt.Errorf("%s: HTTP %d，读取错误响应失败: %w", providerName, response.StatusCode, readErr)
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
		message = http.StatusText(response.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", providerName, response.StatusCode, message)
}

func allowEmptyAPIKey(host string) bool {
	host = strings.ToLower(host)
	return strings.HasPrefix(host, "localhost") ||
		strings.HasPrefix(host, "127.0.0.1") ||
		strings.HasPrefix(host, "[::1]")
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

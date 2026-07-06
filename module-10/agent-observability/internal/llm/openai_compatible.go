package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// OpenAICompatibleConfig 描述一个 OpenAI Chat Completions 兼容 Provider。
type OpenAICompatibleConfig struct {
	Name         string
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// OpenAICompatibleProvider 通过 Chat Completions 协议调用真实模型服务。
type OpenAICompatibleProvider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewOpenAICompatibleProvider 创建真实模型 Provider。
func NewOpenAICompatibleProvider(config OpenAICompatibleConfig) (*OpenAICompatibleProvider, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "deepseek"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("无效 baseURL: %q", baseURL)
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("API key 不能为空，请设置 DEEPSEEK_API_KEY 或 LLM_API_KEY")
	}
	model := strings.TrimSpace(config.DefaultModel)
	if model == "" {
		model = "deepseek-chat"
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAICompatibleProvider{
		name:         name,
		baseURL:      baseURL,
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   httpClient,
	}, nil
}

// NewOpenAICompatibleFromEnv 从环境变量创建真实模型 Provider。
func NewOpenAICompatibleFromEnv() (*OpenAICompatibleProvider, error) {
	config := OpenAICompatibleConfig{
		Name:         firstNonEmpty(os.Getenv("LLM_PROVIDER_NAME"), "deepseek"),
		BaseURL:      firstNonEmpty(os.Getenv("LLM_BASE_URL"), os.Getenv("DEEPSEEK_BASE_URL"), "https://api.deepseek.com"),
		APIKey:       firstNonEmpty(os.Getenv("LLM_API_KEY"), os.Getenv("DEEPSEEK_API_KEY")),
		DefaultModel: firstNonEmpty(os.Getenv("LLM_MODEL"), os.Getenv("DEEPSEEK_MODEL"), "deepseek-chat"),
	}
	return NewOpenAICompatibleProvider(config)
}

// Name 返回 provider 名称。
func (provider *OpenAICompatibleProvider) Name() string { return provider.name }

// DefaultModel 返回 provider 默认模型。
func (provider *OpenAICompatibleProvider) DefaultModel() string { return provider.defaultModel }

// Chat 调用真实 OpenAI-compatible Chat Completions 接口。
func (provider *OpenAICompatibleProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if err := ValidateRequest(req); err != nil {
		return ChatResponse{}, err
	}
	model, err := EffectiveModel(req.Model, provider.defaultModel)
	if err != nil {
		return ChatResponse{}, err
	}
	wireRequest := struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Temperature *float64  `json:"temperature,omitempty"`
		MaxTokens   *int      `json:"max_tokens,omitempty"`
		Stream      bool      `json:"stream,omitempty"`
	}{
		Model:       model,
		Messages:    append([]Message(nil), req.Messages...),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}
	body, err := json.Marshal(wireRequest)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("序列化模型请求: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("创建模型请求: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)

	httpResponse, err := provider.httpClient.Do(httpRequest)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("调用模型接口: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return ChatResponse{}, parseAPIError(provider.name, httpResponse)
	}

	var payload struct {
		ID      string `json:"id"`
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
	if err := json.NewDecoder(httpResponse.Body).Decode(&payload); err != nil {
		return ChatResponse{}, fmt.Errorf("解析模型响应: %w", err)
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("模型响应中没有 choices")
	}
	if payload.Model == "" {
		payload.Model = model
	}
	outputTokens := payload.Usage.CompletionTokens
	if outputTokens == 0 && payload.Usage.TotalTokens > payload.Usage.PromptTokens {
		outputTokens = payload.Usage.TotalTokens - payload.Usage.PromptTokens
	}
	return ChatResponse{
		ID:      payload.ID,
		Model:   payload.Model,
		Content: payload.Choices[0].Message.Content,
		Usage: Usage{
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

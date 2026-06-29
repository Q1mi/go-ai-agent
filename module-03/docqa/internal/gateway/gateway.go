package gateway

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

	"github.com/q1mi/docqa-context/internal/llm"
	"github.com/q1mi/docqa-context/internal/transport"
)

// ProviderConfig 描述一个 OpenAI 兼容模型供应商。
//
// BaseURL 使用平台的 API 根地址，例如 https://api.deepseek.com 或
// https://api.openai.com/v1。网关会在后面拼接 /chat/completions。
type ProviderConfig struct {
	Name         string
	BaseURL      string
	APIKey       string
	DefaultModel string
}

// Config 是大模型网关的启动配置。
//
// Providers 的顺序就是故障转移顺序；Client 为空时使用 M01 风格的默认
// transport.Client。
type Config struct {
	Providers []ProviderConfig
	Client    *transport.Client
}

// Gateway 是 M02 建立的大模型调用入口。
//
// M03 的 docqa 命令只调用 Gateway.Chat，具体模型平台、鉴权方式和
// OpenAI 兼容协议细节都由网关内部处理。
type Gateway struct {
	providers []llm.Provider
}

// New 按配置构造网关。
func New(config Config) (*Gateway, error) {
	if len(config.Providers) == 0 {
		return nil, errors.New("至少需要配置一个 Provider")
	}
	client := config.Client
	if client == nil {
		client = transport.NewClient()
	}
	providers := make([]llm.Provider, 0, len(config.Providers))
	for _, providerConfig := range config.Providers {
		provider, err := newOpenAICompatibleProvider(providerConfig, client)
		if err != nil {
			return nil, fmt.Errorf("构造 Provider %q: %w", providerConfig.Name, err)
		}
		providers = append(providers, provider)
	}
	return &Gateway{providers: providers}, nil
}

// NewFromEnv 从环境变量读取 Provider 配置并构造网关。
func NewFromEnv() (*Gateway, error) {
	config, err := LoadFromEnv()
	if err != nil {
		return nil, err
	}
	return New(config)
}

// LoadFromEnv 读取课程练习约定的环境变量。
//
// 支持两类配置：
//   - 通用 LLM_BASE_URL / LLM_API_KEY / LLM_MODEL；
//   - M02 网关风格的 DEEPSEEK_*、OPENAI_*、DOUBAO_*、OLLAMA_*。
func LoadFromEnv() (Config, error) {
	var providers []ProviderConfig
	addProvider := func(name, prefix, defaultBaseURL string, apiKeyOptional bool) error {
		baseURL := strings.TrimSpace(os.Getenv(prefix + "_BASE_URL"))
		if baseURL == "" {
			baseURL = defaultBaseURL
		}
		apiKey := strings.TrimSpace(os.Getenv(prefix + "_API_KEY"))
		model := strings.TrimSpace(os.Getenv(prefix + "_MODEL"))
		if apiKey == "" && model == "" {
			return nil
		}
		if model == "" {
			return fmt.Errorf("%s_MODEL 不能为空", prefix)
		}
		if apiKey == "" && !apiKeyOptional {
			return fmt.Errorf("%s_API_KEY 不能为空", prefix)
		}
		providers = append(providers, ProviderConfig{
			Name:         name,
			BaseURL:      baseURL,
			APIKey:       apiKey,
			DefaultModel: model,
		})
		return nil
	}

	if err := addProvider(envOrDefault("LLM_PROVIDER_NAME", "llm"), "LLM", "https://api.deepseek.com", false); err != nil {
		return Config{}, err
	}
	if err := addProvider("deepseek", "DEEPSEEK", "https://api.deepseek.com", false); err != nil {
		return Config{}, err
	}
	if err := addProvider("openai", "OPENAI", "https://api.openai.com/v1", false); err != nil {
		return Config{}, err
	}
	if err := addProvider("doubao", "DOUBAO", "https://ark.cn-beijing.volces.com/api/v3", false); err != nil {
		return Config{}, err
	}
	if err := addProvider("ollama", "OLLAMA", "http://localhost:11434/v1", true); err != nil {
		return Config{}, err
	}
	if len(providers) == 0 {
		return Config{}, errors.New("未启用任何 Provider，请配置 LLM_API_KEY/LLM_MODEL 或 M02 网关环境变量")
	}
	return Config{Providers: providers}, nil
}

// Chat 按 Provider 顺序尝试调用模型。
//
// 当前面的 Provider 失败时，网关会继续尝试下一个 Provider，实现 M02
// 课后练习要求的基础故障转移。
func (gateway *Gateway) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if gateway == nil || len(gateway.providers) == 0 {
		return nil, errors.New("网关没有可用 Provider")
	}
	var providerErrors []error
	for _, provider := range gateway.providers {
		response, err := provider.Chat(ctx, req)
		if err == nil {
			return response, nil
		}
		providerErrors = append(providerErrors, fmt.Errorf("%s: %w", provider.Name(), err))
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("所有 Provider 均失败: %w", errors.Join(providerErrors...))
}

// Providers 返回当前网关中的 Provider 名称，便于日志和调试。
func (gateway *Gateway) Providers() []string {
	if gateway == nil {
		return nil
	}
	names := make([]string, 0, len(gateway.providers))
	for _, provider := range gateway.providers {
		names = append(names, provider.Name())
	}
	return names
}

// openAICompatibleProvider 是网关内部的 OpenAI 兼容 Provider 实现。
type openAICompatibleProvider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	client       *transport.Client
}

// newOpenAICompatibleProvider 校验配置并创建 Provider。
func newOpenAICompatibleProvider(config ProviderConfig, client *transport.Client) (*openAICompatibleProvider, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "openai-compatible"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("baseURL 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("无效 baseURL: %q", baseURL)
	}
	defaultModel := strings.TrimSpace(config.DefaultModel)
	if defaultModel == "" {
		return nil, errors.New("defaultModel 不能为空")
	}
	if client == nil {
		client = transport.NewClient()
	}
	return &openAICompatibleProvider{
		name:         name,
		baseURL:      baseURL,
		apiKey:       strings.TrimSpace(config.APIKey),
		defaultModel: defaultModel,
		client:       client,
	}, nil
}

// Name 返回 Provider 标识。
func (provider *openAICompatibleProvider) Name() string {
	return provider.name
}

// DefaultModel 返回 Provider 默认模型。
func (provider *openAICompatibleProvider) DefaultModel() string {
	return provider.defaultModel
}

// Capabilities 返回 Provider 能力。
func (provider *openAICompatibleProvider) Capabilities() llm.Capability {
	return llm.Capability{}
}

// Chat 调用 OpenAI 兼容的 /chat/completions 接口。
func (provider *openAICompatibleProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	model, err := llm.EffectiveModel(req.Model, provider.defaultModel)
	if err != nil {
		return nil, err
	}
	wireRequest := struct {
		Model       string        `json:"model"`
		Messages    []llm.Message `json:"messages"`
		Temperature *float64      `json:"temperature,omitempty"`
		MaxTokens   *int          `json:"max_tokens,omitempty"`
		Stream      bool          `json:"stream,omitempty"`
	}{
		Model:       model,
		Messages:    append([]llm.Message(nil), req.Messages...),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}
	body, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, fmt.Errorf("序列化请求: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		provider.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("创建请求: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	if provider.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}

	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("调用接口: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, parseAPIError(provider.name, response)
	}

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
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析响应: %w", err)
	}
	if len(payload.Choices) == 0 {
		return nil, errors.New("模型响应中没有 choices")
	}
	if payload.Model == "" {
		payload.Model = model
	}
	outputTokens := payload.Usage.CompletionTokens
	if outputTokens == 0 && payload.Usage.TotalTokens > payload.Usage.PromptTokens {
		outputTokens = payload.Usage.TotalTokens - payload.Usage.PromptTokens
	}
	return &llm.ChatResponse{
		Content:      payload.Choices[0].Message.Content,
		Model:        payload.Model,
		FinishReason: payload.Choices[0].FinishReason,
		Usage: llm.Usage{
			InputTokens:  payload.Usage.PromptTokens,
			OutputTokens: outputTokens,
		},
	}, nil
}

// ChatStream 保留 M02 Provider 接口中的流式能力入口。
func (provider *openAICompatibleProvider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("%s: M03 文档问答练习暂未实现 ChatStream", provider.name)
}

// parseAPIError 解析 OpenAI 兼容接口的错误响应。
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

// envOrDefault 读取环境变量，空值时使用默认值。
func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

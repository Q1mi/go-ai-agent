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

	"github.com/q1mi/mcptools/internal/llm"
)

// Config 描述 OpenAI 兼容 Provider 配置。
type Config struct {
	Name         string
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
	ToolCalling  bool
}

// Provider 支持普通 Chat 和 Function Calling。
type Provider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
	toolCalling  bool
}

// NewFromEnv 从 LLM_* 环境变量创建 Provider。
func NewFromEnv(toolCalling bool) (*Provider, error) {
	return New(Config{
		Name:         envOrDefault("LLM_PROVIDER_NAME", "openai-compatible"),
		BaseURL:      envOrDefault("LLM_BASE_URL", "https://api.deepseek.com"),
		APIKey:       strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		DefaultModel: strings.TrimSpace(os.Getenv("LLM_MODEL")),
		ToolCalling:  toolCalling,
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
		client = &http.Client{}
	}
	return &Provider{
		name:         name,
		baseURL:      baseURL,
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   client,
		toolCalling:  config.ToolCalling,
	}, nil
}

// Name 返回 Provider 名称。
func (provider *Provider) Name() string { return provider.name }

// Capabilities 返回 Provider 能力。
func (provider *Provider) Capabilities() llm.Capability {
	return llm.Capability{Tools: provider.toolCalling}
}

// Chat 调用 OpenAI 兼容 /chat/completions。
func (provider *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = provider.defaultModel
	}
	wireRequest := openAIChatRequest{
		Model:    model,
		Messages: toOpenAIMessages(req.Messages),
		Stream:   false,
		Stop:     req.Stop,
	}
	if provider.toolCalling && len(req.Tools) > 0 {
		wireRequest.Tools = toOpenAITools(req.Tools)
		wireRequest.ToolChoice = "auto"
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
	return parseChatResponse(response.Body)
}

// ChatStream 保留接口入口，本练习未实现流式。
func (provider *Provider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("%s: M06 练习暂未实现 ChatStream", provider.name)
}

type openAIChatRequest struct {
	Model      string          `json:"model"`
	Messages   []openAIMessage `json:"messages"`
	Tools      []openAITool    `json:"tools,omitempty"`
	ToolChoice string          `json:"tool_choice,omitempty"`
	Stream     bool            `json:"stream,omitempty"`
	Stop       []string        `json:"stop,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}

type openAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

func toOpenAIMessages(messages []llm.Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(messages))
	for _, message := range messages {
		item := openAIMessage{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
		}
		for _, call := range message.ToolCalls {
			item.ToolCalls = append(item.ToolCalls, openAIToolCall{
				ID:   call.ID,
				Type: "function",
				Function: openAIFunction{
					Name:      call.Name,
					Arguments: string(call.Args),
				},
			})
		}
		out = append(out, item)
	}
	return out
}

func toOpenAITools(tools []llm.ToolDef) []openAITool {
	out := make([]openAITool, 0, len(tools))
	for _, item := range tools {
		out = append(out, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        item.Name,
				Description: item.Description,
				Parameters:  item.Parameters,
			},
		})
	}
	return out
}

func parseChatResponse(body io.Reader) (*llm.ChatResponse, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content   string           `json:"content"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
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
	message := payload.Choices[0].Message
	toolCalls := make([]llm.ToolCall, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		args := json.RawMessage(strings.TrimSpace(call.Function.Arguments))
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		toolCalls = append(toolCalls, llm.ToolCall{ID: call.ID, Name: call.Function.Name, Args: args})
	}
	outputTokens := payload.Usage.CompletionTokens
	if outputTokens == 0 && payload.Usage.TotalTokens > payload.Usage.PromptTokens {
		outputTokens = payload.Usage.TotalTokens - payload.Usage.PromptTokens
	}
	return &llm.ChatResponse{
		Content:      message.Content,
		ToolCalls:    toolCalls,
		InputTokens:  payload.Usage.PromptTokens,
		OutputTokens: outputTokens,
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

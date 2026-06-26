package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/q1mi/llmrouter/internal/llm"
	"github.com/q1mi/llmrouter/internal/transport"
)

const defaultAnthropicVersion = "2023-06-01"

type Config struct {
	Name             string
	BaseURL          string
	APIKey           string
	DefaultModel     string
	AnthropicVersion string
	Capabilities     llm.Capability
	Client           *transport.Client
}

type Provider struct {
	name             string
	baseURL          string
	apiKey           string
	defaultModel     string
	anthropicVersion string
	capabilities     llm.Capability
	client           *transport.Client
}

func New(config Config) (*Provider, error) {
	if strings.TrimSpace(config.Name) == "" {
		config.Name = "claude"
	}
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" {
		config.BaseURL = "https://api.anthropic.com/v1"
	}
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("无效的 Claude base URL: %q", config.BaseURL)
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("Claude API key 不能为空")
	}
	if strings.TrimSpace(config.DefaultModel) == "" {
		return nil, errors.New("Claude default model 不能为空")
	}
	if config.AnthropicVersion == "" {
		config.AnthropicVersion = defaultAnthropicVersion
	}
	if config.Client == nil {
		config.Client = transport.NewClient()
	}
	if config.Capabilities == (llm.Capability{}) {
		config.Capabilities.Streaming = true
	}

	return &Provider{
		name:             strings.TrimSpace(config.Name),
		baseURL:          config.BaseURL,
		apiKey:           strings.TrimSpace(config.APIKey),
		defaultModel:     strings.TrimSpace(config.DefaultModel),
		anthropicVersion: config.AnthropicVersion,
		capabilities:     config.Capabilities,
		client:           config.Client,
	}, nil
}

func (provider *Provider) Name() string {
	return provider.name
}

func (provider *Provider) DefaultModel() string {
	return provider.defaultModel
}

func (provider *Provider) Capabilities() llm.Capability {
	return provider.capabilities
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (provider *Provider) Chat(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	response, err := provider.doRequest(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var payload anthropicResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s: 解析响应: %w", provider.name, err)
	}
	var content strings.Builder
	for _, block := range payload.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}
	if content.Len() == 0 {
		return nil, fmt.Errorf("%s: 响应中没有 text content block", provider.name)
	}

	model := payload.Model
	if model == "" {
		model, _ = llm.EffectiveModel(request.Model, provider.defaultModel)
	}
	return &llm.ChatResponse{
		Content:      content.String(),
		Model:        model,
		FinishReason: payload.StopReason,
		Usage: llm.Usage{
			InputTokens:  payload.Usage.InputTokens,
			OutputTokens: payload.Usage.OutputTokens,
		},
	}, nil
}

func (provider *Provider) ChatStream(
	ctx context.Context,
	request llm.ChatRequest,
) (<-chan llm.StreamChunk, error) {
	if !provider.capabilities.Streaming {
		return nil, fmt.Errorf("%s 不支持流式输出", provider.name)
	}
	response, err := provider.doRequest(ctx, request, true)
	if err != nil {
		return nil, err
	}

	output := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(output)
		defer response.Body.Close()

		var usage llm.Usage
		err := transport.ParseSSE(ctx, response.Body, func(event transport.SSEEvent) error {
			chunk, done, err := parseStreamEvent(event.Data, &usage)
			if err != nil {
				return err
			}
			if done {
				if usage != (llm.Usage{}) {
					usageCopy := usage
					if err := sendChunk(ctx, output, llm.StreamChunk{Usage: &usageCopy}); err != nil {
						return err
					}
				}
				return errStreamDone
			}
			if chunk.Content == "" {
				return nil
			}
			return sendChunk(ctx, output, chunk)
		})
		switch {
		case errors.Is(err, errStreamDone):
			_ = sendChunk(ctx, output, llm.StreamChunk{Done: true})
		case err != nil:
			_ = sendChunk(ctx, output, llm.StreamChunk{Err: err})
		default:
			_ = sendChunk(ctx, output, llm.StreamChunk{
				Err: errors.New("Claude 流在 message_stop 之前结束"),
			})
		}
	}()
	return output, nil
}

func (provider *Provider) doRequest(
	ctx context.Context,
	request llm.ChatRequest,
	stream bool,
) (*http.Response, error) {
	wireRequest, err := provider.adaptRequest(request, stream)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, fmt.Errorf("%s: 序列化请求: %w", provider.name, err)
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		provider.baseURL+"/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: 创建请求: %w", provider.name, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-api-key", provider.apiKey)
	httpRequest.Header.Set("anthropic-version", provider.anthropicVersion)
	if stream {
		httpRequest.Header.Set("Accept", "text/event-stream")
	} else {
		httpRequest.Header.Set("Accept", "application/json")
	}

	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("%s: 调用接口: %w", provider.name, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		defer response.Body.Close()
		return nil, parseAPIError(provider.name, response)
	}
	return response, nil
}

func (provider *Provider) adaptRequest(
	request llm.ChatRequest,
	stream bool,
) (anthropicRequest, error) {
	if err := llm.ValidateRequest(request); err != nil {
		return anthropicRequest{}, err
	}
	if request.Temperature != nil && *request.Temperature > 1 {
		return anthropicRequest{}, errors.New("Claude temperature 必须在 [0, 1] 范围内")
	}
	model, err := llm.EffectiveModel(request.Model, provider.defaultModel)
	if err != nil {
		return anthropicRequest{}, err
	}

	var systems []string
	messages := make([]anthropicMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		switch message.Role {
		case llm.RoleSystem:
			systems = append(systems, message.Content)
		case llm.RoleUser, llm.RoleAssistant:
			messages = append(messages, anthropicMessage{
				Role:    string(message.Role),
				Content: message.Content,
			})
		default:
			return anthropicRequest{}, fmt.Errorf(
				"Claude 适配器暂不支持 role %q",
				message.Role,
			)
		}
	}
	if len(messages) == 0 {
		return anthropicRequest{}, errors.New("Claude 请求至少需要一条 user/assistant 消息")
	}

	maxTokens := 1024
	if request.MaxTokens != nil {
		maxTokens = *request.MaxTokens
	}
	return anthropicRequest{
		Model:       model,
		System:      strings.Join(systems, "\n\n"),
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: request.Temperature,
		Stream:      stream,
	}, nil
}

func parseAPIError(providerName string, response *http.Response) error {
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if readErr != nil {
		return &llm.APIError{
			Provider:   providerName,
			StatusCode: response.StatusCode,
			Message:    "读取错误响应失败: " + readErr.Error(),
		}
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
	return &llm.APIError{
		Provider:   providerName,
		StatusCode: response.StatusCode,
		Message:    message,
	}
}

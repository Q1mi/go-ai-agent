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
	"strings"

	"github.com/q1mi/llmrouter/internal/llm"
	"github.com/q1mi/llmrouter/internal/transport"
)

type Config struct {
	Name         string
	BaseURL      string
	APIKey       string
	DefaultModel string
	Capabilities llm.Capability
	Client       *transport.Client
}

type Provider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	capabilities llm.Capability
	client       *transport.Client
}

func New(config Config) (*Provider, error) {
	config.Name = strings.TrimSpace(config.Name)
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.DefaultModel = strings.TrimSpace(config.DefaultModel)
	if config.Name == "" {
		return nil, errors.New("Provider name 不能为空")
	}
	if err := validateBaseURL(config.BaseURL); err != nil {
		return nil, err
	}
	if config.DefaultModel == "" {
		return nil, errors.New("Provider default model 不能为空")
	}
	if config.Client == nil {
		config.Client = transport.NewClient()
	}
	if config.Capabilities == (llm.Capability{}) {
		config.Capabilities.Streaming = true
	}

	return &Provider{
		name:         config.Name,
		baseURL:      config.BaseURL,
		apiKey:       strings.TrimSpace(config.APIKey),
		defaultModel: config.DefaultModel,
		capabilities: config.Capabilities,
		client:       config.Client,
	}, nil
}

func validateBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("无效的 Provider base URL: %q", value)
	}
	return nil
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

type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []llm.Message  `json:"messages"`
	Temperature   *float64       `json:"temperature,omitempty"`
	MaxTokens     *int           `json:"max_tokens,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage usage `json:"usage"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (provider *Provider) Chat(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	response, err := provider.doRequest(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var payload chatResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s: 解析响应: %w", provider.name, err)
	}
	if len(payload.Choices) == 0 {
		return nil, fmt.Errorf("%s: 模型响应中没有 choices", provider.name)
	}

	model := payload.Model
	if model == "" {
		model, _ = llm.EffectiveModel(request.Model, provider.defaultModel)
	}
	return &llm.ChatResponse{
		Content:      payload.Choices[0].Message.Content,
		Model:        model,
		FinishReason: payload.Choices[0].FinishReason,
		Usage: llm.Usage{
			InputTokens:  payload.Usage.PromptTokens,
			OutputTokens: payload.Usage.CompletionTokens,
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

		sawDone := false
		err := transport.ParseSSE(ctx, response.Body, func(event transport.SSEEvent) error {
			chunk, done, err := parseStreamEvent(event.Data)
			if err != nil {
				return err
			}
			if done {
				sawDone = true
				return errStreamDone
			}
			if chunk.Content == "" && chunk.Usage == nil {
				return nil
			}
			return sendChunk(ctx, output, chunk)
		})
		switch {
		case errors.Is(err, errStreamDone):
			_ = sendChunk(ctx, output, llm.StreamChunk{Done: true})
		case err != nil:
			_ = sendChunk(ctx, output, llm.StreamChunk{Err: err})
		case !sawDone:
			_ = sendChunk(ctx, output, llm.StreamChunk{
				Err: errors.New("OpenAI 兼容流在 [DONE] 之前结束"),
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
	if err := llm.ValidateRequest(request); err != nil {
		return nil, err
	}
	for _, message := range request.Messages {
		if message.Role == llm.RoleTool {
			return nil, errors.New("M02 OpenAI 适配器尚未实现 tool message 协议")
		}
	}
	model, err := llm.EffectiveModel(request.Model, provider.defaultModel)
	if err != nil {
		return nil, err
	}

	wireRequest := chatRequest{
		Model:       model,
		Messages:    append([]llm.Message(nil), request.Messages...),
		Temperature: request.Temperature,
		MaxTokens:   request.MaxTokens,
		Stream:      stream,
	}
	if stream {
		wireRequest.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	body, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, fmt.Errorf("%s: 序列化请求: %w", provider.name, err)
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		provider.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: 创建请求: %w", provider.name, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	if stream {
		httpRequest.Header.Set("Accept", "text/event-stream")
	}
	if provider.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
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
			Type    string `json:"type"`
			Code    any    `json:"code"`
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

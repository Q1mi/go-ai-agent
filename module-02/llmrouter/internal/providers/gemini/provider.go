package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

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
	if strings.TrimSpace(config.Name) == "" {
		config.Name = "gemini"
	}
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("无效的 Gemini base URL: %q", config.BaseURL)
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("Gemini API key 不能为空")
	}
	if strings.TrimSpace(config.DefaultModel) == "" {
		return nil, errors.New("Gemini default model 不能为空")
	}
	if config.Client == nil {
		config.Client = transport.NewClient()
	}
	if config.Capabilities == (llm.Capability{}) {
		config.Capabilities.Streaming = true
	}

	return &Provider{
		name:         strings.TrimSpace(config.Name),
		baseURL:      config.BaseURL,
		apiKey:       strings.TrimSpace(config.APIKey),
		defaultModel: strings.TrimSpace(config.DefaultModel),
		capabilities: config.Capabilities,
		client:       config.Client,
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

type generateContentRequest struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content       content `json:"content"`
		FinishReason  string  `json:"finishReason"`
		FinishMessage string  `json:"finishMessage"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	UsageMetadata usageMetadata `json:"usageMetadata"`
	ModelVersion  string        `json:"modelVersion"`
	Error         *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func (provider *Provider) Chat(ctx context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
	response, err := provider.doRequest(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var payload generateContentResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s: 解析响应: %w", provider.name, err)
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return nil, fmt.Errorf("%s: 模型返回错误: %s", provider.name, payload.Error.Message)
	}
	if len(payload.Candidates) == 0 {
		if payload.PromptFeedback != nil && payload.PromptFeedback.BlockReason != "" {
			return nil, fmt.Errorf("%s: 请求被拦截: %s", provider.name, payload.PromptFeedback.BlockReason)
		}
		return nil, fmt.Errorf("%s: 模型响应中没有 candidates", provider.name)
	}

	candidate := payload.Candidates[0]
	text := textFromContent(candidate.Content)
	if text == "" {
		reason := strings.TrimSpace(candidate.FinishReason)
		if candidate.FinishMessage != "" {
			reason = strings.TrimSpace(reason + ": " + candidate.FinishMessage)
		}
		if reason != "" {
			return nil, fmt.Errorf("%s: 响应中没有 text part，finishReason=%s", provider.name, reason)
		}
		return nil, fmt.Errorf("%s: 响应中没有 text part", provider.name)
	}

	model := payload.ModelVersion
	if model == "" {
		model, _ = llm.EffectiveModel(request.Model, provider.defaultModel)
	}
	return &llm.ChatResponse{
		Content:      text,
		Model:        model,
		FinishReason: candidate.FinishReason,
		Usage:        usageFromMetadata(payload.UsageMetadata),
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
			if chunk.Content != "" || chunk.Usage != nil {
				if err := sendChunk(ctx, output, chunk); err != nil {
					return err
				}
			}
			if done {
				sawDone = true
				return errStreamDone
			}
			return nil
		})
		switch {
		case errors.Is(err, errStreamDone):
			_ = sendChunk(ctx, output, llm.StreamChunk{Done: true})
		case err != nil:
			_ = sendChunk(ctx, output, llm.StreamChunk{Err: err})
		case !sawDone:
			_ = sendChunk(ctx, output, llm.StreamChunk{Done: true})
		}
	}()
	return output, nil
}

func (provider *Provider) doRequest(
	ctx context.Context,
	request llm.ChatRequest,
	stream bool,
) (*http.Response, error) {
	wireRequest, model, err := provider.adaptRequest(request)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, fmt.Errorf("%s: 序列化请求: %w", provider.name, err)
	}

	endpoint, err := provider.endpoint(model, stream)
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: 创建请求: %w", provider.name, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-goog-api-key", provider.apiKey)
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

func (provider *Provider) adaptRequest(request llm.ChatRequest) (generateContentRequest, string, error) {
	if err := llm.ValidateRequest(request); err != nil {
		return generateContentRequest{}, "", err
	}
	model, err := llm.EffectiveModel(request.Model, provider.defaultModel)
	if err != nil {
		return generateContentRequest{}, "", err
	}

	var systemParts []part
	contents := make([]content, 0, len(request.Messages))
	for _, message := range request.Messages {
		switch message.Role {
		case llm.RoleSystem:
			systemParts = append(systemParts, part{Text: message.Content})
		case llm.RoleUser:
			contents = append(contents, content{
				Role:  "user",
				Parts: []part{{Text: message.Content}},
			})
		case llm.RoleAssistant:
			contents = append(contents, content{
				Role:  "model",
				Parts: []part{{Text: message.Content}},
			})
		default:
			return generateContentRequest{}, "", fmt.Errorf(
				"Gemini 适配器暂不支持 role %q",
				message.Role,
			)
		}
	}
	if len(contents) == 0 {
		return generateContentRequest{}, "", errors.New("Gemini 请求至少需要一条 user/model 消息")
	}

	wireRequest := generateContentRequest{Contents: contents}
	if len(systemParts) > 0 {
		wireRequest.SystemInstruction = &content{Parts: systemParts}
	}
	if request.Temperature != nil || request.MaxTokens != nil {
		wireRequest.GenerationConfig = &generationConfig{
			Temperature:     request.Temperature,
			MaxOutputTokens: request.MaxTokens,
		}
	}
	return wireRequest, model, nil
}

func (provider *Provider) endpoint(model string, stream bool) (string, error) {
	modelName := strings.TrimSpace(model)
	modelName = strings.TrimPrefix(modelName, "models/")
	if modelName == "" {
		return "", errors.New("Gemini model 不能为空")
	}
	method := "generateContent"
	if stream {
		method = "streamGenerateContent"
	}
	endpoint := fmt.Sprintf("%s/models/%s:%s", provider.baseURL, url.PathEscape(modelName), method)
	if !stream {
		return endpoint, nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("%s: 构造流式 URL: %w", provider.name, err)
	}
	query := parsed.Query()
	query.Set("alt", "sse")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func textFromContent(value content) string {
	var builder strings.Builder
	for _, part := range value.Parts {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func usageFromMetadata(metadata usageMetadata) llm.Usage {
	outputTokens := metadata.CandidatesTokenCount
	if outputTokens == 0 && metadata.TotalTokenCount > metadata.PromptTokenCount {
		outputTokens = metadata.TotalTokenCount - metadata.PromptTokenCount
	}
	return llm.Usage{
		InputTokens:  metadata.PromptTokenCount,
		OutputTokens: outputTokens,
	}
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
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error.Message != "" {
		message = payload.Error.Message
		if payload.Error.Status != "" {
			message += " (" + payload.Error.Status + ")"
		}
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

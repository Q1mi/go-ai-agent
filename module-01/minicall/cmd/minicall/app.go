package main

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

	"github.com/q1mi/minicall/internal/llm"
	"github.com/q1mi/minicall/internal/transport"
)

const requestTimeout = 2 * time.Minute

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func loadConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("LLM_BASE_URL")), "/"),
		APIKey:  strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		Model:   strings.TrimSpace(os.Getenv("LLM_MODEL")),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	if c.BaseURL == "" {
		missing = append(missing, "LLM_BASE_URL")
	}
	if c.APIKey == "" {
		missing = append(missing, "LLM_API_KEY")
	}
	if c.Model == "" {
		missing = append(missing, "LLM_MODEL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("缺少环境变量: %s", strings.Join(missing, ", "))
	}

	parsed, err := url.Parse(c.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("LLM_BASE_URL 不是有效的 HTTP(S) 地址: %q", c.BaseURL)
	}
	return nil
}

func runOnce(parent context.Context, cfg Config, question string) error {
	ctx, cancel := context.WithTimeout(parent, requestTimeout)
	defer cancel()

	resp, err := callModel(ctx, transport.NewClient(), cfg, question)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, resp.Content)
	fmt.Fprintf(
		os.Stdout,
		"\ntoken: input=%d output=%d total=%d\n",
		resp.InputTokens,
		resp.OutputTokens,
		resp.TotalTokens,
	)
	return nil
}

func callModel(
	ctx context.Context,
	client httpDoer,
	cfg Config,
	question string,
) (*llm.ChatResponse, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("HTTP 客户端不能为空")
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, errors.New("问题不能为空")
	}

	payload := llm.ChatRequest{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: question},
		},
		Stream: false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化模型请求: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		cfg.BaseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("创建模型请求: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("模型调用被取消: %w", ctx.Err())
		}
		return nil, fmt.Errorf("调用模型接口: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if readErr != nil {
			return nil, fmt.Errorf("模型接口返回 %s，且读取错误响应失败: %w", resp.Status, readErr)
		}
		message := strings.TrimSpace(string(raw))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("模型接口返回 %s: %s", resp.Status, message)
	}

	result, err := llm.DecodeChatResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	return result, nil
}

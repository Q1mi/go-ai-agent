package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
)

// HTTPConfig 描述底层 http.Transport 的连接池和超时配置。
type HTTPConfig struct {
	DialTimeout         time.Duration
	KeepAlive           time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	TLSHandshakeTimeout time.Duration
}

// DefaultHTTPConfig 返回适合命令行练习使用的默认 HTTP 配置。
func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{
		DialTimeout:         5 * time.Second,
		KeepAlive:           30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
}

// HTTPOption 用于调整 HTTPConfig。
type HTTPOption func(*HTTPConfig)

// WithMaxIdleConnsPerHost 设置每个主机允许保留的最大空闲连接数。
func WithMaxIdleConnsPerHost(maximum int) HTTPOption {
	return func(config *HTTPConfig) { config.MaxIdleConnsPerHost = maximum }
}

// newTransport 根据配置创建底层 http.Transport。
func newTransport(config HTTPConfig) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: time.Second,
	}
}

// NewHTTPClient 创建带连接池配置的标准库 HTTP 客户端。
func NewHTTPClient(options ...HTTPOption) *http.Client {
	config := DefaultHTTPConfig()
	for _, option := range options {
		option(&config)
	}
	return &http.Client{Transport: newTransport(config)}
}

// RetryConfig 描述请求重试策略。
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig 返回默认重试策略。
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   10 * time.Second,
	}
}

// Limiter 是请求限流器接口。
type Limiter interface {
	Wait(context.Context) error
}

// Client 是带重试和可选限流的 HTTP 客户端封装。
type Client struct {
	httpClient *http.Client
	retry      RetryConfig
	limiter    Limiter
}

// Option 用于调整 Client。
type Option func(*Client)

// WithRetry 设置 Client 的重试策略。
func WithRetry(config RetryConfig) Option {
	return func(client *Client) { client.retry = config }
}

// WithHTTPClient 注入自定义标准库 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

// WithLimiter 为 Client 注入限流器。
func WithLimiter(limiter Limiter) Option {
	return func(client *Client) { client.limiter = limiter }
}

// NewClient 创建课程中统一使用的 HTTP 客户端。
func NewClient(options ...Option) *Client {
	client := &Client{
		httpClient: NewHTTPClient(),
		retry:      DefaultRetryConfig(),
	}
	for _, option := range options {
		option(client)
	}
	client.retry = normalizeRetryConfig(client.retry)
	return client
}

// Do 发送 HTTP 请求，并对可重试错误做指数退避重试。
func (client *Client) Do(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, errors.New("request 不能为空")
	}
	if client == nil || client.httpClient == nil {
		return nil, errors.New("transport client 未初始化")
	}

	ctx := request.Context()
	if client.limiter != nil {
		if err := client.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("等待限流器: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= client.retry.MaxRetries; attempt++ {
		attemptRequest, err := requestForAttempt(request, attempt)
		if err != nil {
			return nil, err
		}
		response, err := client.httpClient.Do(attemptRequest)
		if err == nil && !retryableStatus(response.StatusCode) {
			return response, nil
		}

		var wait time.Duration
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			wait = backoff(attempt, client.retry)
		default:
			wait = retryAfter(response)
			if wait <= 0 {
				wait = backoff(attempt, client.retry)
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
			_ = response.Body.Close()
			lastErr = fmt.Errorf("服务端返回 %s", response.Status)
		}

		if attempt == client.retry.MaxRetries {
			break
		}
		if !sleep(ctx, wait) {
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", client.retry.MaxRetries, lastErr)
}

// requestForAttempt 为重试构造可安全发送的请求副本。
func requestForAttempt(request *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 {
		return request, nil
	}
	if request.Body == nil {
		return request.Clone(request.Context()), nil
	}
	if request.GetBody == nil {
		return nil, errors.New("请求体无法重放，不能安全重试")
	}
	body, err := request.GetBody()
	if err != nil {
		return nil, fmt.Errorf("重建请求体: %w", err)
	}
	cloned := request.Clone(request.Context())
	cloned.Body = body
	return cloned, nil
}

// retryableStatus 判断 HTTP 状态码是否适合重试。
func retryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

// normalizeRetryConfig 规整非法的重试配置，避免出现负数延迟或倒置区间。
func normalizeRetryConfig(config RetryConfig) RetryConfig {
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.BaseDelay < 0 {
		config.BaseDelay = 0
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = config.BaseDelay
	}
	if config.MaxDelay < config.BaseDelay {
		config.MaxDelay = config.BaseDelay
	}
	return config
}

// backoff 计算当前重试轮次的退避等待时间，并加入少量 jitter。
func backoff(attempt int, config RetryConfig) time.Duration {
	if config.BaseDelay <= 0 {
		return 0
	}
	delay := config.BaseDelay
	for i := 0; i < attempt && delay < config.MaxDelay; i++ {
		if delay > config.MaxDelay/2 {
			delay = config.MaxDelay
			break
		}
		delay *= 2
	}
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}
	jitterRange := delay / 5
	if jitterRange <= 0 {
		return delay
	}
	return delay - delay/10 + time.Duration(rand.Int63n(int64(jitterRange)))
}

// retryAfter 从 HTTP Retry-After 头中解析服务端建议的等待时间。
func retryAfter(response *http.Response) time.Duration {
	if response == nil {
		return 0
	}
	value := response.Header.Get("Retry-After")
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay < 0 {
			return 0
		}
		return delay
	}
	return 0
}

// sleep 等待指定时间，并在 context 取消时提前返回。
func sleep(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

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

type HTTPConfig struct {
	DialTimeout         time.Duration
	KeepAlive           time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	TLSHandshakeTimeout time.Duration
}

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

type HTTPOption func(*HTTPConfig)

func WithDialTimeout(timeout time.Duration) HTTPOption {
	return func(config *HTTPConfig) { config.DialTimeout = timeout }
}

func WithMaxIdleConnsPerHost(maximum int) HTTPOption {
	return func(config *HTTPConfig) { config.MaxIdleConnsPerHost = maximum }
}

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

func NewHTTPClient(options ...HTTPOption) *http.Client {
	config := DefaultHTTPConfig()
	for _, option := range options {
		option(&config)
	}
	return &http.Client{Transport: newTransport(config)}
}

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   10 * time.Second,
	}
}

type Limiter interface {
	Wait(context.Context) error
}

type Client struct {
	httpClient *http.Client
	retry      RetryConfig
	limiter    Limiter
}

type Option func(*Client)

func WithRetry(config RetryConfig) Option {
	return func(client *Client) { client.retry = config }
}

func WithLimiter(limiter Limiter) Option {
	return func(client *Client) { client.limiter = limiter }
}

func WithHTTPOptions(options ...HTTPOption) Option {
	return func(client *Client) { client.httpClient = NewHTTPClient(options...) }
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

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

func retryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

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

func retryAfter(response *http.Response) time.Duration {
	return retryAfterAt(response, time.Now())
}

func retryAfterAt(response *http.Response, now time.Time) time.Duration {
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
	if retryAt, err := http.ParseTime(value); err == nil {
		if delay := retryAt.Sub(now); delay > 0 {
			return delay
		}
	}
	return 0
}

func sleep(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
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

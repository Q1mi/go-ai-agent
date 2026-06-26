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

func WithDialTimeout(d time.Duration) HTTPOption {
	return func(c *HTTPConfig) { c.DialTimeout = d }
}

func WithMaxIdleConnsPerHost(n int) HTTPOption {
	return func(c *HTTPConfig) { c.MaxIdleConnsPerHost = n }
}

func newTransport(c HTTPConfig) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   c.DialTimeout,
			KeepAlive: c.KeepAlive,
		}).DialContext,
		MaxIdleConns:          c.MaxIdleConns,
		MaxIdleConnsPerHost:   c.MaxIdleConnsPerHost,
		IdleConnTimeout:       c.IdleConnTimeout,
		TLSHandshakeTimeout:   c.TLSHandshakeTimeout,
		ExpectContinueTimeout: time.Second,
	}
}

// NewHTTPClient 不设置整体 Timeout。每次请求的生命周期由 context 控制，
// 这样后续接入 SSE 长连接时不会被固定超时误杀。
func NewHTTPClient(opts ...HTTPOption) *http.Client {
	cfg := DefaultHTTPConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &http.Client{Transport: newTransport(cfg)}
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
	Wait(ctx context.Context) error
}

type Client struct {
	http    *http.Client
	retry   RetryConfig
	limiter Limiter
}

type Option func(*Client)

func WithRetry(cfg RetryConfig) Option {
	return func(c *Client) { c.retry = cfg }
}

func WithLimiter(l Limiter) Option {
	return func(c *Client) { c.limiter = l }
}

func WithHTTPOptions(opts ...HTTPOption) Option {
	return func(c *Client) { c.http = NewHTTPClient(opts...) }
}

// WithHTTPClient 主要用于测试或接入自定义 RoundTripper。
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.http = client
		}
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		http:  NewHTTPClient(),
		retry: DefaultRetryConfig(),
	}
	for _, opt := range opts {
		opt(c)
	}
	c.retry = normalizeRetryConfig(c.retry)
	return c
}

// Do 执行请求，并对网络错误、429 和 5xx 进行指数退避重试。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("request 不能为空")
	}
	if c == nil || c.http == nil {
		return nil, errors.New("transport client 未初始化")
	}

	ctx := req.Context()
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("等待限流器: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		attemptReq, err := requestForAttempt(req, attempt)
		if err != nil {
			return nil, err
		}

		resp, err := c.http.Do(attemptReq)
		if err == nil && !retryableStatus(resp.StatusCode) {
			return resp, nil
		}

		var wait time.Duration
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			wait = backoff(attempt, c.retry)
		default:
			wait = retryAfter(resp)
			if wait <= 0 {
				wait = backoff(attempt, c.retry)
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("服务端返回 %s", resp.Status)
		}

		if attempt == c.retry.MaxRetries {
			break
		}
		if !sleep(ctx, wait) {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", c.retry.MaxRetries, lastErr)
}

func requestForAttempt(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 {
		return req, nil
	}
	if req.Body == nil {
		return req.Clone(req.Context()), nil
	}
	if req.GetBody == nil {
		return nil, errors.New("请求体无法重放，不能安全重试")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("重建请求体: %w", err)
	}
	clone := req.Clone(req.Context())
	clone.Body = body
	return clone, nil
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func normalizeRetryConfig(cfg RetryConfig) RetryConfig {
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.BaseDelay < 0 {
		cfg.BaseDelay = 0
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = cfg.BaseDelay
	}
	if cfg.MaxDelay < cfg.BaseDelay {
		cfg.MaxDelay = cfg.BaseDelay
	}
	return cfg
}

func backoff(attempt int, cfg RetryConfig) time.Duration {
	if cfg.BaseDelay <= 0 {
		return 0
	}

	delay := cfg.BaseDelay
	for i := 0; i < attempt && delay < cfg.MaxDelay; i++ {
		if delay > cfg.MaxDelay/2 {
			delay = cfg.MaxDelay
			break
		}
		delay *= 2
	}
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}

	jitterRange := delay / 5
	if jitterRange <= 0 {
		return delay
	}
	return delay - delay/10 + time.Duration(rand.Int63n(int64(jitterRange)))
}

func retryAfter(resp *http.Response) time.Duration {
	return retryAfterAt(resp, time.Now())
}

func retryAfterAt(resp *http.Response, now time.Time) time.Duration {
	if resp == nil {
		return 0
	}
	value := resp.Header.Get("Retry-After")
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

package transport

import (
	"fmt"
	"net/http"
	"time"
)

// retryTransport 是一层中间件式的 RoundTripper：先限流，再带重试地委托给底层 base。
type retryTransport struct {
	base    http.RoundTripper
	retry   RetryConfig
	limiter Limiter // 可空
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if req.Body != nil {
		defer req.Body.Close() // RoundTripper 负责关闭传入 Body；重试用 GetBody 取独立副本
	}
	if t.limiter != nil {
		if err := t.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}
	var lastErr error
	for attempt := 0; attempt <= t.retry.MaxRetries; attempt++ {
		// RoundTripper 契约：不应改动传入的 req（它可能被并发读）。
		// 每次重试都 Clone 一份、用 GetBody 重建一次性的 Body。
		r := req
		if req.Body != nil && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r = req.Clone(ctx)
			r.Body = body
		}
		resp, err := t.base.RoundTrip(r) // 委托给真正发请求的底层 transport
		var wait time.Duration
		switch {
		case err != nil:
			lastErr, wait = err, backoff(attempt, t.retry)
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			if wait = retryAfter(resp); wait <= 0 {
				wait = backoff(attempt, t.retry)
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("服务端返回 %d", resp.StatusCode)
		default:
			return resp, nil
		}
		if attempt == t.retry.MaxRetries {
			break
		}
		if !sleep(ctx, wait) {
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", t.retry.MaxRetries, lastErr)
}

// NewRetryHTTPClient 返回一个“自带重试/限流”的普通 *http.Client，调用方照常用即可。
func NewRetryHTTPClient(retry RetryConfig, limiter Limiter, opts ...HTTPOption) *http.Client {
	cfg := DefaultHTTPConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &http.Client{
		Transport: &retryTransport{
			base:    newTransport(cfg),
			retry:   retry,
			limiter: limiter,
		},
	}
}

package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, time.June, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		val  string
		want time.Duration
	}{
		{name: "无头", want: 0},
		{name: "秒数", val: "3", want: 3 * time.Second},
		{name: "HTTP date", val: now.Add(5 * time.Second).Format(http.TimeFormat), want: 5 * time.Second},
		{name: "过去的 HTTP date", val: now.Add(-time.Second).Format(http.TimeFormat), want: 0},
		{name: "负数", val: "-1", want: 0},
		{name: "非法值", val: "soon", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: make(http.Header)}
			if tt.val != "" {
				resp.Header.Set("Retry-After", tt.val)
			}
			if got := retryAfterAt(resp, now); got != tt.want {
				t.Fatalf("retryAfterAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientDoRetriesAndReplaysBody(t *testing.T) {
	var hits atomic.Int32
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取请求体: %v", err)
			return
		}
		bodies = append(bodies, string(body))
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 1
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = time.Millisecond

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		server.URL,
		strings.NewReader(`{"message":"hello"}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := NewClient(WithRetry(cfg)).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("请求次数 = %d, want 2", got)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("重试请求体不一致: %#v", bodies)
	}
}

func TestClientDoDoesNotRetryNormalClientError(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("请求次数 = %d, want 1", got)
	}
}

func TestClientDoRetriesServerError(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 1
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = time.Millisecond

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient(WithRetry(cfg)).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("请求次数 = %d, want 2", got)
	}
}

func TestClientDoStopsWhenContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewClient().Do(req)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context deadline exceeded", err)
	}
}

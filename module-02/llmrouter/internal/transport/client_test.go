package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientRetries429AndReplaysBody(t *testing.T) {
	var hits atomic.Int32
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll: %v", err)
			return
		}
		bodies = append(bodies, string(body))
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retry := DefaultRetryConfig()
	retry.MaxRetries = 1
	retry.BaseDelay = time.Millisecond
	retry.MaxDelay = time.Millisecond
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		server.URL,
		strings.NewReader(`{"hello":"world"}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	response, err := NewClient(WithRetry(retry)).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2", hits.Load())
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("bodies = %#v", bodies)
	}
}

func TestRetryAfterAt(t *testing.T) {
	now := time.Date(2026, time.June, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "empty"},
		{name: "seconds", value: "3", want: 3 * time.Second},
		{name: "date", value: now.Add(5 * time.Second).Format(http.TimeFormat), want: 5 * time.Second},
		{name: "invalid", value: "later"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &http.Response{Header: make(http.Header)}
			response.Header.Set("Retry-After", test.value)
			if got := retryAfterAt(response, now); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

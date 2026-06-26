package router

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/q1mi/llmrouter/internal/cost"
	"github.com/q1mi/llmrouter/internal/llm"
)

type Candidate struct {
	Provider    llm.Provider
	Pricing     cost.Pricing
	LatencyHint time.Duration
	index       int
}

type Stats struct {
	Count int
	P50   time.Duration
	P95   time.Duration
}

type Strategy interface {
	Name() string
	Order(candidates []Candidate, stats map[string]Stats) []Candidate
}

type Result struct {
	Response *llm.ChatResponse
	Provider string
	Pricing  cost.Pricing
	Duration time.Duration
}

type StreamResult struct {
	Chunks   <-chan llm.StreamChunk
	Provider string
	Pricing  cost.Pricing
}

type Router struct {
	candidates []Candidate
	strategy   Strategy

	mu      sync.Mutex
	samples map[string][]time.Duration
}

func New(strategy Strategy, candidates ...Candidate) (*Router, error) {
	if strategy == nil {
		return nil, errors.New("router strategy 不能为空")
	}
	if len(candidates) == 0 {
		return nil, errors.New("至少需要一个 Provider")
	}

	seen := make(map[string]struct{}, len(candidates))
	cloned := make([]Candidate, len(candidates))
	for i, candidate := range candidates {
		if candidate.Provider == nil {
			return nil, fmt.Errorf("candidate[%d] 的 Provider 为空", i)
		}
		name := strings.TrimSpace(candidate.Provider.Name())
		if name == "" {
			return nil, fmt.Errorf("candidate[%d] 的 Provider 名称为空", i)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("Provider 名称重复: %s", name)
		}
		if err := candidate.Pricing.Validate(); err != nil {
			return nil, fmt.Errorf("%s pricing: %w", name, err)
		}
		seen[name] = struct{}{}
		candidate.index = i
		cloned[i] = candidate
	}
	return &Router{
		candidates: cloned,
		strategy:   strategy,
		samples:    make(map[string][]time.Duration),
	}, nil
}

func (router *Router) Chat(ctx context.Context, request llm.ChatRequest) (*Result, error) {
	if err := llm.ValidateRequest(request); err != nil {
		return nil, err
	}

	ordered := router.strategy.Order(router.candidates, router.Stats())
	var providerErrors []error
	for _, candidate := range ordered {
		startedAt := time.Now()
		response, err := candidate.Provider.Chat(ctx, request)
		duration := time.Since(startedAt)
		router.record(candidate.Provider.Name(), duration)
		if err == nil && response != nil {
			return &Result{
				Response: response,
				Provider: candidate.Provider.Name(),
				Pricing:  candidate.Pricing,
				Duration: duration,
			}, nil
		}
		if err == nil {
			err = errors.New("Provider 返回了 nil response")
		}
		providerErrors = append(
			providerErrors,
			fmt.Errorf("%s: %w", candidate.Provider.Name(), err),
		)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("所有 Provider 均失败: %w", errors.Join(providerErrors...))
}

func (router *Router) ChatStream(
	ctx context.Context,
	request llm.ChatRequest,
) (*StreamResult, error) {
	if err := llm.ValidateRequest(request); err != nil {
		return nil, err
	}

	ordered := router.strategy.Order(router.candidates, router.Stats())
	var providerErrors []error
	for _, candidate := range ordered {
		if !candidate.Provider.Capabilities().Streaming {
			providerErrors = append(
				providerErrors,
				fmt.Errorf("%s: 不支持流式输出", candidate.Provider.Name()),
			)
			continue
		}

		startedAt := time.Now()
		chunks, err := candidate.Provider.ChatStream(ctx, request)
		if err == nil && chunks == nil {
			err = errors.New("Provider 返回了 nil stream")
		}
		if err != nil {
			router.record(candidate.Provider.Name(), time.Since(startedAt))
			providerErrors = append(
				providerErrors,
				fmt.Errorf("%s: %w", candidate.Provider.Name(), err),
			)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}

		proxied := make(chan llm.StreamChunk, 16)
		go func(
			providerName string,
			input <-chan llm.StreamChunk,
			streamStartedAt time.Time,
		) {
			defer close(proxied)
			defer func() {
				router.record(providerName, time.Since(streamStartedAt))
			}()
			for chunk := range input {
				select {
				case <-ctx.Done():
					return
				case proxied <- chunk:
				}
			}
		}(candidate.Provider.Name(), chunks, startedAt)

		return &StreamResult{
			Chunks:   proxied,
			Provider: candidate.Provider.Name(),
			Pricing:  candidate.Pricing,
		}, nil
	}
	return nil, fmt.Errorf("所有 Provider 均无法建立流: %w", errors.Join(providerErrors...))
}

func (router *Router) StrategyName() string {
	return router.strategy.Name()
}

func (router *Router) Stats() map[string]Stats {
	router.mu.Lock()
	defer router.mu.Unlock()

	result := make(map[string]Stats, len(router.candidates))
	for _, candidate := range router.candidates {
		name := candidate.Provider.Name()
		samples := append([]time.Duration(nil), router.samples[name]...)
		result[name] = calculateStats(samples)
	}
	return result
}

func (router *Router) record(providerName string, duration time.Duration) {
	router.mu.Lock()
	defer router.mu.Unlock()

	const maxSamples = 256
	samples := append(router.samples[providerName], duration)
	if len(samples) > maxSamples {
		samples = append([]time.Duration(nil), samples[len(samples)-maxSamples:]...)
	}
	router.samples[providerName] = samples
}

func calculateStats(samples []time.Duration) Stats {
	if len(samples) == 0 {
		return Stats{}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return Stats{
		Count: len(samples),
		P50:   percentile(samples, 0.50),
		P95:   percentile(samples, 0.95),
	}
}

func percentile(sorted []time.Duration, quantile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(sorted))*quantile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

package obs

import "sync"

// Metrics 是课堂演示用的内存指标。
type Metrics struct {
	mu             sync.Mutex
	Requests       map[string]int
	InputTokens    int
	OutputTokens   int
	StepsHistogram map[int]int
}

// MetricsSnapshot 是指标快照，避免把锁暴露给调用方。
type MetricsSnapshot struct {
	Requests       map[string]int
	InputTokens    int
	OutputTokens   int
	StepsHistogram map[int]int
}

// NewMetrics 创建指标容器。
func NewMetrics() *Metrics {
	return &Metrics{Requests: map[string]int{}, StepsHistogram: map[int]int{}}
}

// IncRequest 记录一次请求。
func (metrics *Metrics) IncRequest(intent, status string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.Requests[intent+":"+status]++
}

// AddTokens 累加 token。
func (metrics *Metrics) AddTokens(input, output int) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.InputTokens += input
	metrics.OutputTokens += output
}

// ObserveSteps 记录步数。
func (metrics *Metrics) ObserveSteps(steps int) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.StepsHistogram[steps]++
}

// Snapshot 返回指标快照。
func (metrics *Metrics) Snapshot() MetricsSnapshot {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	out := MetricsSnapshot{
		Requests:       map[string]int{},
		InputTokens:    metrics.InputTokens,
		OutputTokens:   metrics.OutputTokens,
		StepsHistogram: map[int]int{},
	}
	for k, v := range metrics.Requests {
		out.Requests[k] = v
	}
	for k, v := range metrics.StepsHistogram {
		out.StepsHistogram[k] = v
	}
	return out
}

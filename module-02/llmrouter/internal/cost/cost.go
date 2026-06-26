package cost

import (
	"fmt"
	"sync"

	"github.com/q1mi/llmrouter/internal/llm"
)

type Pricing struct {
	InputPer1M  float64
	OutputPer1M float64
	Currency    string
}

func (p Pricing) Validate() error {
	if p.InputPer1M < 0 || p.OutputPer1M < 0 {
		return fmt.Errorf("token 单价不能为负数")
	}
	return nil
}

func (p Pricing) Configured() bool {
	return p.InputPer1M > 0 || p.OutputPer1M > 0
}

func Estimate(usage llm.Usage, pricing Pricing) float64 {
	return float64(usage.InputTokens)/1e6*pricing.InputPer1M +
		float64(usage.OutputTokens)/1e6*pricing.OutputPer1M
}

type Snapshot struct {
	Usage llm.Usage
	Cost  float64
}

type Accumulator struct {
	mu    sync.Mutex
	usage llm.Usage
	cost  float64
}

func (a *Accumulator) Add(usage llm.Usage, pricing Pricing) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.usage.InputTokens += usage.InputTokens
	a.usage.OutputTokens += usage.OutputTokens
	a.cost += Estimate(usage, pricing)
}

func (a *Accumulator) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Snapshot{Usage: a.usage, Cost: a.cost}
}

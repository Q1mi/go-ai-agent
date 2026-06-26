package cost

import (
	"math"
	"sync"
	"testing"

	"github.com/q1mi/llmrouter/internal/llm"
)

func TestEstimate(t *testing.T) {
	got := Estimate(
		llm.Usage{InputTokens: 500_000, OutputTokens: 250_000},
		Pricing{InputPer1M: 2, OutputPer1M: 4},
	)
	if math.Abs(got-2) > 1e-9 {
		t.Fatalf("Estimate() = %f, want 2", got)
	}
}

func TestAccumulatorConcurrent(t *testing.T) {
	var accumulator Accumulator
	pricing := Pricing{InputPer1M: 1, OutputPer1M: 2}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			accumulator.Add(llm.Usage{InputTokens: 10, OutputTokens: 5}, pricing)
		}()
	}
	wg.Wait()

	snapshot := accumulator.Snapshot()
	if snapshot.Usage.InputTokens != 1000 || snapshot.Usage.OutputTokens != 500 {
		t.Fatalf("Usage = %+v", snapshot.Usage)
	}
}

package main

import (
	"testing"

	"github.com/q1mi/llmrouter/internal/cost"
	"github.com/q1mi/llmrouter/internal/llm"
)

func TestConfiguredCost(t *testing.T) {
	pricing := cost.Pricing{
		InputPer1M:  1,
		OutputPer1M: 2,
		Currency:    "USD",
	}
	got := cost.Estimate(llm.Usage{InputTokens: 1000, OutputTokens: 500}, pricing)
	if got != 0.002 {
		t.Fatalf("cost = %f, want 0.002", got)
	}
}

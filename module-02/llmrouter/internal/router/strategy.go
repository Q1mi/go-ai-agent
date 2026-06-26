package router

import (
	"math"
	"sort"
	"time"
)

type Priority struct{}

func (Priority) Name() string {
	return "priority"
}

func (Priority) Order(candidates []Candidate, _ map[string]Stats) []Candidate {
	return append([]Candidate(nil), candidates...)
}

type CheapestFirst struct{}

func (CheapestFirst) Name() string {
	return "cheapest"
}

func (CheapestFirst) Order(candidates []Candidate, _ map[string]Stats) []Candidate {
	ordered := append([]Candidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return priceScore(ordered[i]) < priceScore(ordered[j])
	})
	return ordered
}

func priceScore(candidate Candidate) float64 {
	if !candidate.Pricing.Configured() {
		return math.Inf(1)
	}
	return candidate.Pricing.InputPer1M + candidate.Pricing.OutputPer1M
}

type LowestLatency struct{}

func (LowestLatency) Name() string {
	return "latency"
}

func (LowestLatency) Order(candidates []Candidate, stats map[string]Stats) []Candidate {
	ordered := append([]Candidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return latencyScore(ordered[i], stats) < latencyScore(ordered[j], stats)
	})
	return ordered
}

func latencyScore(candidate Candidate, stats map[string]Stats) time.Duration {
	if stat := stats[candidate.Provider.Name()]; stat.Count > 0 {
		return stat.P50
	}
	if candidate.LatencyHint > 0 {
		return candidate.LatencyHint
	}
	return time.Duration(math.MaxInt64)
}

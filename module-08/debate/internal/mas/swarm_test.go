package mas

import (
	"context"
	"strings"
	"testing"
)

func TestSwarmRun(t *testing.T) {
	swarm := &Swarm{
		MaxHops: 4,
		Agents: map[string]SwarmAgent{
			"frontdesk": {
				Name: "frontdesk",
				Run: func(ctx context.Context, input string) (SwarmResult, error) {
					return SwarmResult{Answer: "识别为架构问题", HandoffTo: "architect"}, nil
				},
			},
			"architect": {
				Name: "architect",
				Run: func(ctx context.Context, input string) (SwarmResult, error) {
					if !strings.Contains(input, "识别为架构问题") {
						t.Fatalf("handoff context missing, input=%q", input)
					}
					return SwarmResult{Answer: "建议先明确模块边界"}, nil
				},
			},
		},
	}
	got, err := swarm.Run(context.Background(), "frontdesk", "怎么选架构？")
	if err != nil {
		t.Fatal(err)
	}
	if got != "建议先明确模块边界" {
		t.Fatalf("Swarm.Run()=%q", got)
	}
}

func TestSwarmLoopGuard(t *testing.T) {
	swarm := &Swarm{
		MaxHops: 5,
		Agents: map[string]SwarmAgent{
			"a": {Name: "a", Run: func(ctx context.Context, input string) (SwarmResult, error) {
				return SwarmResult{HandoffTo: "b"}, nil
			}},
			"b": {Name: "b", Run: func(ctx context.Context, input string) (SwarmResult, error) {
				return SwarmResult{HandoffTo: "a"}, nil
			}},
		},
	}
	_, err := swarm.Run(context.Background(), "a", "loop")
	if err == nil {
		t.Fatalf("expected loop guard error")
	}
}

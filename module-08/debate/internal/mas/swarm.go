package mas

import (
	"context"
	"fmt"
)

// SwarmResult 表示某个 Agent 的处理结果。
type SwarmResult struct {
	Answer    string // 非空表示给出最终答案或阶段性处理结果
	HandoffTo string // 非空表示转交给该 Agent
}

// SwarmAgent 是 Swarm 中的一个节点。
type SwarmAgent struct {
	Name string
	Run  func(ctx context.Context, input string) (SwarmResult, error)
}

// Swarm 沿 handoff 链执行，直到产生最终答案或达到最大转交次数。
type Swarm struct {
	Agents  map[string]SwarmAgent
	MaxHops int
}

// Run 从 start 指定的 Agent 开始执行。
func (swarm *Swarm) Run(ctx context.Context, start, input string) (string, error) {
	if swarm.MaxHops <= 0 {
		swarm.MaxHops = 8
	}
	cur := start
	visited := map[string]int{}
	for hop := 0; hop < swarm.MaxHops; hop++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		agent, ok := swarm.Agents[cur]
		if !ok {
			return "", fmt.Errorf("不存在的 Agent: %q", cur)
		}
		visited[cur]++
		if visited[cur] > 2 {
			return "", fmt.Errorf("检测到重复转交: %s", cur)
		}
		res, err := agent.Run(ctx, input)
		if err != nil {
			return "", err
		}
		if res.HandoffTo == "" {
			return res.Answer, nil
		}
		cur = res.HandoffTo
		if res.Answer != "" {
			input += "\n\n[" + agent.Name + " 的处理]：" + res.Answer
		}
	}
	return "", fmt.Errorf("转交超过 %d 次仍无最终答案", swarm.MaxHops)
}

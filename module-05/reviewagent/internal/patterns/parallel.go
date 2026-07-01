package patterns

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/q1mi/reviewagent/internal/llm"
)

// Sectioning 并行执行互不依赖的任务，按原顺序返回结果。
//
// 任一任务失败时会取消同批次其他任务，并返回第一个错误。
func Sectioning(ctx context.Context, tasks []func(context.Context) (string, error)) ([]string, error) {
	results := make([]string, len(tasks))
	var wg sync.WaitGroup
	var firstErr error
	var once sync.Once

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, task := range tasks {
		if task == nil {
			return nil, fmt.Errorf("tasks[%d] 为空", i)
		}
		wg.Add(1)
		go func(i int, task func(context.Context) (string, error)) {
			defer wg.Done()
			out, err := task(cctx)
			if err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
				return
			}
			results[i] = out
		}(i, task)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// Voting 用相同提示词独立跑 n 次，返回 n 个结果，交给上层聚合。
func Voting(ctx context.Context, provider llm.Provider, model, system, user string, n int) ([]string, error) {
	if n <= 0 {
		return nil, fmt.Errorf("n 必须大于 0")
	}
	tasks := make([]func(context.Context) (string, error), n)
	for i := range tasks {
		tasks[i] = func(ctx context.Context) (string, error) {
			return Complete(ctx, provider, model, system, user)
		}
	}
	return Sectioning(ctx, tasks)
}

// Majority 返回出现次数最多的结果，适合有限类别投票。
func Majority(votes []string) string {
	counts := make(map[string]int)
	best, bestN := "", 0
	for _, vote := range votes {
		vote = strings.TrimSpace(vote)
		counts[vote]++
		if counts[vote] > bestN {
			best, bestN = vote, counts[vote]
		}
	}
	return best
}

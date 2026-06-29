package plan

import (
	"context"
	"fmt"
	"sync"

	"github.com/q1mi/assistant/internal/tool"
)

// Execute 对应课件 4.9 的按层执行器。
// 每一层内部并行执行；同层任一任务失败时取消本层剩余任务。
func Execute(ctx context.Context, plan Plan, registry *tool.Registry) (map[string]string, error) {
	levels, err := Levels(plan)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]Task, len(plan.Tasks))
	for _, task := range plan.Tasks {
		byID[task.ID] = task
	}

	results := make(map[string]string)
	var mu sync.Mutex
	for _, level := range levels {
		levelCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		var once sync.Once
		var firstErr error

		for _, id := range level {
			task := byID[id]
			wg.Add(1)
			go func() {
				defer wg.Done()
				item, ok := registry.Get(task.Tool)
				if !ok {
					once.Do(func() {
						firstErr = fmt.Errorf("工具 %q 不存在", task.Tool)
						cancel()
					})
					return
				}
				out, err := item.Call(levelCtx, task.Args)
				if err != nil {
					once.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
				mu.Lock()
				results[task.ID] = out
				mu.Unlock()
			}()
		}
		wg.Wait()
		cancel()
		if firstErr != nil {
			return results, firstErr
		}
	}
	return results, nil
}

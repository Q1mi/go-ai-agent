package mas

import (
	"context"
	"sync"
)

// Stage 是流水线的一个阶段：多个 worker 从 in 读取，处理后写入 out。
func Stage[I, O any](ctx context.Context, in <-chan I, workers int, fn func(context.Context, I) (O, error)) <-chan O {
	if workers <= 0 {
		workers = 1
	}
	out := make(chan O)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-in:
					if !ok {
						return
					}
					next, err := fn(ctx, item)
					if err != nil {
						continue
					}
					select {
					case out <- next:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

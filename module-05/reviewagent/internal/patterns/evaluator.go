package patterns

import (
	"context"
	"fmt"
)

// Evaluation 是评估者的结构化输出。
type Evaluation struct {
	Pass     bool   `json:"pass"`
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// Generator 根据上一轮反馈生成内容；首轮 feedback 为空。
type Generator func(ctx context.Context, feedback string) (string, error)

// Evaluator 评估一份内容，给出是否达标和反馈。
type Evaluator func(ctx context.Context, output string) (Evaluation, error)

// EvaluatorOptimizer 执行“生成→评估→带反馈重生成”的有界循环。
func EvaluatorOptimizer(ctx context.Context, gen Generator, eval Evaluator, maxRounds int) (string, Evaluation, error) {
	if maxRounds <= 0 {
		return "", Evaluation{}, fmt.Errorf("maxRounds 必须大于 0")
	}
	var output string
	var ev Evaluation
	feedback := ""
	for round := 0; round < maxRounds; round++ {
		var err error
		output, err = gen(ctx, feedback)
		if err != nil {
			return "", ev, err
		}
		ev, err = eval(ctx, output)
		if err != nil {
			return "", ev, err
		}
		if ev.Pass {
			return output, ev, nil
		}
		feedback = ev.Feedback
	}
	return output, ev, nil
}

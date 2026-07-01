package patterns

import (
	"context"
	"fmt"
	"strings"

	"github.com/q1mi/reviewagent/internal/llm"
)

// Route 是 IntentRouter 的一个目标处理器。
type Route struct {
	Name        string
	Description string
	Handle      func(ctx context.Context, input string) (string, error)
}

// IntentRouter 先分类用户输入，再把请求分发给匹配的处理器。
type IntentRouter struct {
	Provider llm.Provider
	Model    string
	Routes   []Route
	Fallback func(ctx context.Context, input string) (string, error)
}

type routeChoice struct {
	Route string `json:"route"`
}

// Dispatch 分类并分发。分类失败或分类结果无匹配时走 Fallback。
func (router *IntentRouter) Dispatch(ctx context.Context, input string) (string, error) {
	name, err := router.Classify(ctx, input)
	if err == nil {
		for _, route := range router.Routes {
			if route.Name == name {
				if route.Handle == nil {
					return "", fmt.Errorf("路由 %q 缺少处理函数", route.Name)
				}
				return route.Handle(ctx, input)
			}
		}
	}
	if router.Fallback != nil {
		return router.Fallback(ctx, input)
	}
	return "", fmt.Errorf("无法路由，且未配置兜底（意图=%q, err=%v）", name, err)
}

// Classify 只执行意图分类，方便测试和调试。
func (router *IntentRouter) Classify(ctx context.Context, input string) (string, error) {
	if router.Provider == nil {
		return "", fmt.Errorf("router.Provider 不能为空")
	}
	var builder strings.Builder
	for _, route := range router.Routes {
		fmt.Fprintf(&builder, "- %s: %s\n", route.Name, route.Description)
	}
	system := "你是意图分类器。从下列类别中选出最匹配用户输入的一个，" +
		"严格只输出 JSON：{\"route\":\"类别名\"}。\n类别：\n" + builder.String()
	out, err := Complete(ctx, router.Provider, router.Model, system, input)
	if err != nil {
		return "", err
	}
	choice, err := llm.ParseInto[routeChoice](out)
	if err != nil {
		return "", fmt.Errorf("分类输出无法解析: %w", err)
	}
	return strings.TrimSpace(choice.Route), nil
}

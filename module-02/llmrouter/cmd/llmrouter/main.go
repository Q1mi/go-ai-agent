package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/q1mi/llmrouter/internal/appconfig"
	"github.com/q1mi/llmrouter/internal/cost"
	"github.com/q1mi/llmrouter/internal/factory"
	"github.com/q1mi/llmrouter/internal/llm"
	"github.com/q1mi/llmrouter/internal/router"
)

const defaultTimeout = 5 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := appconfig.Load()
	if err != nil {
		return fmt.Errorf("读取配置: %w", err)
	}

	var (
		stream      = config.Stream
		strategy    = config.Strategy
		system      string
		maxTokens   int
		temperature float64
		timeout     time.Duration
	)
	flag.BoolVar(&stream, "stream", stream, "使用流式输出")
	flag.StringVar(&strategy, "strategy", strategy, "路由策略: priority|cheapest|latency")
	flag.StringVar(&system, "system", "", "可选 system message")
	flag.IntVar(&maxTokens, "max-tokens", 0, "最大输出 token，0 表示使用 Provider 默认值")
	flag.Float64Var(&temperature, "temperature", -1, "采样温度，-1 表示使用 Provider 默认值")
	flag.DurationVar(&timeout, "timeout", defaultTimeout, "整次调用超时")
	flag.Parse()

	question := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if question == "" {
		return fmt.Errorf("用法: llmrouter [选项] <你的问题>")
	}
	if maxTokens < 0 {
		return fmt.Errorf("max-tokens 不能为负数")
	}
	if temperature < -1 || temperature > 2 {
		return fmt.Errorf("temperature 必须是 -1 或 [0, 2] 范围内的数字")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout 必须大于 0")
	}

	config.Strategy = strategy
	modelRouter, err := factory.BuildRouter(config)
	if err != nil {
		return err
	}

	messages := make([]llm.Message, 0, 2)
	if strings.TrimSpace(system) != "" {
		messages = append(messages, llm.Message{
			Role:    llm.RoleSystem,
			Content: system,
		})
	}
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: question,
	})
	var options []llm.Option
	if maxTokens > 0 {
		options = append(options, llm.WithMaxTokens(maxTokens))
	}
	if temperature >= 0 {
		options = append(options, llm.WithTemperature(temperature))
	}
	request := llm.NewChatRequest("", messages, options...)

	signalContext, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, timeout)
	defer cancel()

	if stream {
		return runStream(ctx, modelRouter, request)
	}
	return runChat(ctx, modelRouter, request)
}

func runChat(
	ctx context.Context,
	modelRouter *router.Router,
	request llm.ChatRequest,
) error {
	result, err := modelRouter.Chat(ctx, request)
	if err != nil {
		return err
	}

	fmt.Println(result.Response.Content)
	printSummary(
		result.Provider,
		result.Response.Model,
		result.Response.Usage,
		result.Pricing,
		result.Duration,
		modelRouter.Stats()[result.Provider],
	)
	return nil
}

func runStream(
	ctx context.Context,
	modelRouter *router.Router,
	request llm.ChatRequest,
) error {
	startedAt := time.Now()
	result, err := modelRouter.ChatStream(ctx, request)
	if err != nil {
		return err
	}

	fmt.Printf("provider: %s\n\n", result.Provider)
	var usage llm.Usage
	for chunk := range result.Chunks {
		if chunk.Err != nil {
			return chunk.Err
		}
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
	}
	fmt.Println()

	stats := modelRouter.Stats()[result.Provider]
	printSummary(
		result.Provider,
		"",
		usage,
		result.Pricing,
		time.Since(startedAt),
		stats,
	)
	return nil
}

func printSummary(
	providerName string,
	model string,
	usage llm.Usage,
	pricing cost.Pricing,
	duration time.Duration,
	stats router.Stats,
) {
	fmt.Println()
	fmt.Printf("provider: %s\n", providerName)
	if model != "" {
		fmt.Printf("model: %s\n", model)
	}
	fmt.Printf(
		"token: input=%d output=%d total=%d\n",
		usage.InputTokens,
		usage.OutputTokens,
		usage.TotalTokens(),
	)
	if pricing.Configured() {
		fmt.Printf(
			"estimated_cost: %.8f %s\n",
			cost.Estimate(usage, pricing),
			pricing.Currency,
		)
	} else {
		fmt.Println("estimated_cost: 未配置价格")
	}
	fmt.Printf("duration: %s\n", duration.Round(time.Millisecond))
	fmt.Printf(
		"latency_stats: count=%d p50=%s p95=%s\n",
		stats.Count,
		stats.P50.Round(time.Millisecond),
		stats.P95.Round(time.Millisecond),
	)
}

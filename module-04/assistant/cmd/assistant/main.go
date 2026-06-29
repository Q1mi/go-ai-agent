package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/q1mi/assistant/internal/agent"
	"github.com/q1mi/assistant/internal/providers/openai"
	"github.com/q1mi/assistant/internal/tool"
	"github.com/q1mi/assistant/internal/tools/calculator"
	"github.com/q1mi/assistant/internal/tools/now"
)

// main 是 assistant 命令的入口。
func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// run 解析命令行参数，装配 Agent，并根据是否传入目标选择单轮或交互模式。
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("assistant", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		mode      string
		sessionID string
		storeDir  string
		maxSteps  int
		maxTokens int
		timeout   time.Duration
		model     string
	)
	flags.StringVar(&mode, "mode", "function", "运行模式: function|react")
	flags.StringVar(&sessionID, "session", "", "会话 ID；设置后会保存并恢复状态")
	flags.StringVar(&storeDir, "store-dir", ".assistant_sessions", "会话保存目录")
	flags.IntVar(&maxSteps, "max-steps", 10, "单轮最大 Agent 步数")
	flags.IntVar(&maxTokens, "max-tokens", 12000, "单轮累计 token 预算")
	flags.DurationVar(&timeout, "timeout", 2*time.Minute, "单轮运行超时")
	flags.StringVar(&model, "model", "", "模型名；为空时使用 LLM_MODEL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if mode != "function" && mode != "react" {
		return fmt.Errorf("mode 必须是 function 或 react")
	}
	if maxSteps <= 0 {
		return fmt.Errorf("max-steps 必须大于 0")
	}
	if maxTokens <= 0 {
		return fmt.Errorf("max-tokens 必须大于 0")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout 必须大于 0")
	}

	assistant, err := newAssistant(mode, model, sessionID, storeDir, maxSteps, maxTokens)
	if err != nil {
		return err
	}

	goal := strings.TrimSpace(strings.Join(flags.Args(), " "))
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if goal != "" {
		return runOne(signalCtx, assistant, goal, timeout, stdout, stderr)
	}
	return runInteractive(signalCtx, assistant, timeout, stdin, stdout, stderr)
}

// newAssistant 是配套练习的装配层。
// 它把 calculator / now 工具、Provider、Budget、FileStore 组装进 agent.Agent。
func newAssistant(
	mode string,
	model string,
	sessionID string,
	storeDir string,
	maxSteps int,
	maxTokens int,
) (*agent.Agent, error) {
	calculatorTool, err := calculator.New()
	if err != nil {
		return nil, err
	}
	nowTool, err := now.New(time.Local, time.Now)
	if err != nil {
		return nil, err
	}
	registry := tool.NewRegistry(calculatorTool, nowTool)
	provider, err := openai.NewFromEnv(mode == "function")
	if err != nil {
		return nil, err
	}
	budget := agent.DefaultBudget()
	budget.MaxSteps = maxSteps
	budget.MaxTokens = maxTokens

	var opts []agent.Option
	opts = append(opts, agent.WithBudget(budget))
	if strings.TrimSpace(sessionID) != "" {
		opts = append(opts, agent.WithStore(agent.NewFileStore(storeDir), sessionID))
	}
	return agent.New(provider, model, registry, opts...), nil
}

// runInteractive 进入多轮终端交互模式。
//
// 如果设置了 session，Agent 会在每轮之间保存和恢复状态。
func runInteractive(
	ctx context.Context,
	assistant *agent.Agent,
	timeout time.Duration,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	fmt.Fprintln(stdout, "进入交互模式，输入 exit 或 quit 退出。")
	reader := newPromptLineReader(stdin, stdout)
	for {
		fmt.Fprintln(stdout)
		line, err := reader.ReadLine(ctx, "> ")
		if err != nil {
			if isPromptInputDone(err) {
				return nil
			}
			return err
		}
		goal := strings.TrimSpace(line)
		if goal == "" {
			continue
		}
		if goal == "exit" || goal == "quit" {
			return nil
		}
		if err := runOne(ctx, assistant, goal, timeout, stdout, stderr); err != nil {
			return err
		}
	}
}

// runOne 对应课件 4.8 的终端事件消费示例。
func runOne(
	parent context.Context,
	assistant *agent.Agent,
	goal string,
	timeout time.Duration,
	stdout io.Writer,
	stderr io.Writer,
) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	for event := range assistant.RunStream(ctx, goal) {
		switch event.Type {
		case agent.EventThought:
			fmt.Fprintf(stdout, "\n[思考] %s\n", event.Text)
		case agent.EventToolCall:
			fmt.Fprintf(stdout, "[调用工具] %s(%s)\n", event.Tool, event.Args)
		case agent.EventToolResult:
			fmt.Fprintf(stdout, "[工具结果] %s\n", event.Text)
		case agent.EventAnswerDelta:
			fmt.Fprintln(stdout, event.Text)
		case agent.EventError:
			fmt.Fprintln(stderr, "[错误]", event.Text)
		case agent.EventDone:
			fmt.Fprintln(stdout, "[完成]")
		}
	}
	if err := ctx.Err(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

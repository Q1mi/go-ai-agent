package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/q1mi/reviewagent/internal/providers/openai"
	"github.com/q1mi/reviewagent/internal/review"
)

type config struct {
	file       string
	model      string
	format     string
	maxRounds  int
	timeout    time.Duration
	showHelp   bool
	inputParts []string
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "[错误]", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		return err
	}
	if cfg.showHelp {
		return nil
	}
	input, err := readInput(cfg, stdin)
	if err != nil {
		return err
	}

	provider, err := openai.NewFromEnv()
	if err != nil {
		return err
	}
	model := strings.TrimSpace(cfg.model)
	if model == "" {
		model = provider.DefaultModel()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	agent := review.Reviewer{
		Provider:     provider,
		Model:        model,
		MaxRounds:    cfg.maxRounds,
		OutputFormat: cfg.format,
	}
	out, err := agent.AnswerOrReview(ctx, input)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}

func parseFlags(args []string, stderr io.Writer) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("reviewagent", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.file, "file", "", "要审查的 Go 代码文件；为空时读取 stdin 或命令行参数")
	fs.StringVar(&cfg.model, "model", "", "覆盖 LLM_MODEL")
	fs.StringVar(&cfg.format, "format", "markdown", "输出格式：markdown 或 json")
	fs.IntVar(&cfg.maxRounds, "max-rounds", 2, "Evaluator-Optimizer 最大轮次")
	fs.DurationVar(&cfg.timeout, "timeout", 90*time.Second, "单次运行超时时间")
	fs.BoolVar(&cfg.showHelp, "h", false, "显示帮助")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.inputParts = fs.Args()
	if cfg.maxRounds <= 0 {
		return config{}, fmt.Errorf("--max-rounds 必须大于 0")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.format)) {
	case "markdown", "json":
	default:
		return config{}, fmt.Errorf("--format 只能是 markdown 或 json")
	}
	if cfg.showHelp {
		fs.Usage()
	}
	return cfg, nil
}

func readInput(cfg config, stdin io.Reader) (string, error) {
	if strings.TrimSpace(cfg.file) != "" {
		raw, err := os.ReadFile(cfg.file)
		if err != nil {
			return "", fmt.Errorf("读取文件 %s: %w", cfg.file, err)
		}
		return requireInput(string(raw))
	}
	if len(cfg.inputParts) > 0 {
		return requireInput(strings.Join(cfg.inputParts, " "))
	}
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("读取 stdin: %w", err)
	}
	return requireInput(string(raw))
}

func requireInput(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("请输入 Go 代码、普通问题，或使用 --file 指定文件")
	}
	return input, nil
}

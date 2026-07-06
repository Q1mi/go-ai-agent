package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/q1mi/ctxagent/internal/ctxeng"
	"github.com/q1mi/ctxagent/internal/demo"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "[错误]", err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string, out io.Writer) error {
	var rounds int
	var docsDir string
	var memoryDir string
	var csvPath string
	var readMemoryID string
	var historyBudget int
	var toolsBudget int
	var totalBudget int
	var offloadThreshold int
	var keepRecent int
	var maxTools int
	var timeout time.Duration

	fs := flag.NewFlagSet("ctxdemo", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.IntVar(&rounds, "rounds", 8, "模拟对话轮数")
	fs.StringVar(&docsDir, "docs", "./docs", "文档目录")
	fs.StringVar(&memoryDir, "memory", ".ctx-memory", "外置内容目录")
	fs.StringVar(&csvPath, "csv", "", "可选：写出 token 曲线 CSV")
	fs.StringVar(&readMemoryID, "read-memory", "", "读取外置内容 id 后退出")
	fs.IntVar(&historyBudget, "history-budget", 900, "历史 token 预算")
	fs.IntVar(&toolsBudget, "tools-budget", 160, "工具定义 token 预算")
	fs.IntVar(&totalBudget, "total-budget", 1500, "总输入 token 预算")
	fs.IntVar(&offloadThreshold, "offload-threshold", 260, "工具结果超过该 token 时外置")
	fs.IntVar(&keepRecent, "keep-recent", 6, "压缩时保留最近消息数")
	fs.IntVar(&maxTools, "max-tools", 2, "每轮动态暴露工具数量")
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "总超时时间")
	if err := fs.Parse(args); err != nil {
		return err
	}

	memory := &ctxeng.FileMemory{Dir: memoryDir}
	if strings.TrimSpace(readMemoryID) != "" {
		content, err := memory.Read(readMemoryID)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, content)
		return nil
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	result, err := demo.Run(ctx, demo.Config{
		Rounds:           rounds,
		DocsDir:          docsDir,
		MemoryDir:        memoryDir,
		Budget:           ctxeng.Budget{Total: totalBudget, Tools: toolsBudget, History: historyBudget, OutputReserve: 200},
		OffloadThreshold: offloadThreshold,
		KeepRecent:       keepRecent,
		MaxTools:         maxTools,
	})
	if err != nil {
		return err
	}

	printReport(out, result)
	if csvPath != "" {
		if err := writeCSV(csvPath, result.Points); err != nil {
			return err
		}
		fmt.Fprintf(out, "\nCSV 已写入：%s\n", csvPath)
	}
	return nil
}

func printReport(out io.Writer, result demo.Result) {
	fmt.Fprintln(out, "=== Token 曲线对比 ===")
	fmt.Fprintln(out, "| 轮次 | 未治理 total | 已治理 total | 未治理 history | 已治理 history | 动态工具 | 治理动作 |")
	fmt.Fprintln(out, "|---:|---:|---:|---:|---:|---|---|")
	for _, point := range result.Points {
		fmt.Fprintf(out, "| %d | %d | %d | %d | %d | %s | %s |\n",
			point.Round,
			point.RawTokens,
			point.GovernedTokens,
			point.RawHistory,
			point.GovernedHistory,
			strings.Join(point.SelectedTools, ", "),
			joinEvents(point.Events),
		)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "=== ASCII 曲线 ===")
	fmt.Fprintln(out, drawCurve(result.Points))

	if len(result.MemoryIDs) > 0 {
		fmt.Fprintln(out, "=== 外置内容 ===")
		for _, id := range result.MemoryIDs {
			fmt.Fprintf(out, "- %s，可用 `go run ./cmd/ctxdemo -read-memory %s` 读取全文\n", id, id)
		}
	}
}

func joinEvents(events []string) string {
	if len(events) == 0 {
		return "-"
	}
	return strings.Join(events, "<br>")
}

func drawCurve(points []demo.Point) string {
	if len(points) == 0 {
		return ""
	}
	maxValue := 1
	for _, point := range points {
		if point.RawTokens > maxValue {
			maxValue = point.RawTokens
		}
		if point.GovernedTokens > maxValue {
			maxValue = point.GovernedTokens
		}
	}
	var sb strings.Builder
	for _, point := range points {
		rawWidth := scale(point.RawTokens, maxValue, 48)
		governedWidth := scale(point.GovernedTokens, maxValue, 48)
		fmt.Fprintf(&sb, "R%02d raw %5d |%s\n", point.Round, point.RawTokens, strings.Repeat("#", rawWidth))
		fmt.Fprintf(&sb, "R%02d gov %5d |%s\n", point.Round, point.GovernedTokens, strings.Repeat("*", governedWidth))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func scale(value, maxValue, width int) int {
	if value <= 0 {
		return 0
	}
	n := value * width / maxValue
	if n == 0 {
		return 1
	}
	return n
}

func writeCSV(path string, points []demo.Point) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"round", "raw_total", "governed_total", "raw_history", "governed_history"}); err != nil {
		return err
	}
	for _, point := range points {
		if err := writer.Write([]string{
			strconv.Itoa(point.Round),
			strconv.Itoa(point.RawTokens),
			strconv.Itoa(point.GovernedTokens),
			strconv.Itoa(point.RawHistory),
			strconv.Itoa(point.GovernedHistory),
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

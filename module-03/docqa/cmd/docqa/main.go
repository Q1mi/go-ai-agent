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

	"github.com/q1mi/docqa-context/internal/contextpack"
	"github.com/q1mi/docqa-context/internal/gateway"
	"github.com/q1mi/docqa-context/internal/knowledge"
	"github.com/q1mi/docqa-context/internal/llm"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// run 解析命令行参数，检索本地知识库，组装上下文，并按需调用大模型。
func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("docqa", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var (
		product     string
		question    string
		currentTime string
		docsDir     string
		topK        int
		dryRun      bool
		maxTokens   int
		temperature float64
		timeout     time.Duration
	)
	flags.StringVar(&product, "product", "示例网关", "产品名称")
	flags.StringVar(&question, "question", "", "用户问题；也可以放在命令末尾")
	flags.StringVar(&currentTime, "current-time", "", "当前时间，默认使用运行时刻")
	flags.StringVar(&docsDir, "docs", "knowledge", "本地文档知识库目录，支持 .md 和 .txt")
	flags.IntVar(&topK, "top-k", 3, "检索返回的文档数量")
	flags.BoolVar(&dryRun, "dry-run", false, "只打印上下文和预算，不调用模型")
	flags.IntVar(&maxTokens, "max-tokens", 800, "最大输出 token")
	flags.Float64Var(&temperature, "temperature", 0.2, "采样温度")
	flags.DurationVar(&timeout, "timeout", 2*time.Minute, "模型调用超时")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(question) == "" {
		question = strings.TrimSpace(strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(question) == "" {
		return fmt.Errorf("请通过 -question 或命令参数提供问题")
	}
	if strings.TrimSpace(currentTime) == "" {
		currentTime = time.Now().Format(time.RFC3339)
	}
	if topK <= 0 {
		return fmt.Errorf("top-k 必须大于 0")
	}
	if maxTokens <= 0 {
		return fmt.Errorf("max-tokens 必须大于 0")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout 必须大于 0")
	}

	// 加载本地的知识库
	docs, err := knowledge.LoadDir(docsDir)
	if err != nil {
		return err
	}
	// 检索出跟用户问题相关的知识
	hits := knowledge.Retrieve(question, docs, topK)
	// 构建提实词
	plan, err := contextpack.BuildPlan(
		product,
		question,
		currentTime,
		toContextDocuments(hits),
		nil,
	)
	if err != nil {
		return err
	}
	if dryRun {
		return printPlan(out, plan)
	}
	// 调用模型
	answer, err := callModel(plan, maxTokens, temperature, timeout)
	if err != nil {
		return err
	}
	// 打印结果
	return printAnswer(out, answer, hits)
}

// printPlan 输出 dry-run 模式下的上下文方案，帮助学员观察 prompt、预算和缓存排布。
func printPlan(out io.Writer, plan contextpack.Plan) error {
	_, err := fmt.Fprintf(out, `# 文档问答助手上下文方案

## 1. System Prompt

`+"```text\n%s\n```\n\n"+`## 2. 结构化输出 Schema

`+"```json\n%s\n```\n\n"+`## 3. Token 预算

| 部分 | 估算 token | 上限 | 状态 | 缓存位置 | 超限处理 |
| --- | ---: | ---: | --- | --- | --- |
%s

## 4. Prompt Caching 稳定前缀

%s

## 5. 当前时间放置

%s

示例 user message：

`+"```text\n当前时间：%s\n用户问题：%s\n```\n\n"+`## 6. 超长资料处理

%s
`,
		plan.SystemPrompt,
		plan.DifficultySchema,
		formatBudgetRows(plan.Usages),
		formatCachePrefix(plan.CachePrefix),
		plan.TimePlacement,
		plan.CurrentTime,
		plan.Question,
		plan.LongDocsStrategy,
	)
	return err
}

// callModel 通过 M02 网关调用模型。
//
// 这里的 docqa 命令只负责传入 messages 和参数，Provider 选择、OpenAI 兼容请求
// 和故障转移都由 internal/gateway 完成。
func callModel(
	plan contextpack.Plan,
	maxTokens int,
	temperature float64,
	timeout time.Duration,
) (*llm.ChatResponse, error) {
	modelGateway, err := gateway.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("初始化大模型网关: %w", err)
	}
	signalContext, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, timeout)
	defer cancel()
	return modelGateway.Chat(
		ctx,
		llm.NewChatRequest(
			"",
			plan.Messages,
			llm.WithMaxTokens(maxTokens),
			llm.WithTemperature(temperature),
		),
	)
}

// toContextDocuments 把检索命中的知识库文档转换为上下文构建器需要的文档结构。
func toContextDocuments(hits []knowledge.ScoredDocument) []contextpack.Document {
	docs := make([]contextpack.Document, 0, len(hits))
	for i, hit := range hits {
		doc := hit.Document
		id := doc.ID
		if id == "" {
			id = fmt.Sprintf("D%d", i+1)
		}
		docs = append(docs, contextpack.Document{
			ID:      id,
			Title:   doc.Title,
			Source:  doc.Source,
			Content: doc.Content,
		})
	}
	return docs
}

// printAnswer 输出模型回答、token 用量和检索命中的资料，便于核对回答依据。
func printAnswer(
	out io.Writer,
	response *llm.ChatResponse,
	hits []knowledge.ScoredDocument,
) error {
	if response == nil {
		return fmt.Errorf("模型返回空响应")
	}
	fmt.Fprintln(out, response.Content)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "model: %s\n", response.Model)
	fmt.Fprintf(
		out,
		"token: input=%d output=%d total=%d\n",
		response.Usage.InputTokens,
		response.Usage.OutputTokens,
		response.Usage.TotalTokens(),
	)
	fmt.Fprintln(out, "retrieved:")
	for _, hit := range hits {
		fmt.Fprintf(out, "- [%s] %s (%s) score=%d\n", hit.Document.ID, hit.Document.Title, hit.Document.Source, hit.Score)
	}
	return nil
}

// formatBudgetRows 把上下文预算分析结果转换为 Markdown 表格行。
func formatBudgetRows(usages []contextpack.SectionUsage) string {
	var builder strings.Builder
	for _, usage := range usages {
		status := "OK"
		if usage.Overflow {
			status = "超限"
		}
		builder.WriteString(fmt.Sprintf(
			"| %s | %d | %d | %s | %s | %s |\n",
			usage.Name,
			usage.Tokens,
			usage.Limit,
			status,
			usage.CacheHint,
			usage.Resolution,
		))
	}
	return strings.TrimRight(builder.String(), "\n")
}

// formatCachePrefix 把适合 Prompt Caching 的稳定前缀按顺序输出。
func formatCachePrefix(parts []string) string {
	var builder strings.Builder
	for i, part := range parts {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, part))
	}
	return strings.TrimRight(builder.String(), "\n")
}

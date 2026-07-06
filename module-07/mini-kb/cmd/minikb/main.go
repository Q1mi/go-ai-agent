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

	"github.com/q1mi/minikb/internal/agent"
	"github.com/q1mi/minikb/internal/config"
	"github.com/q1mi/minikb/internal/embed"
	"github.com/q1mi/minikb/internal/llm"
	"github.com/q1mi/minikb/internal/providers/openai"
	"github.com/q1mi/minikb/internal/qa"
	"github.com/q1mi/minikb/internal/rag"
	"github.com/q1mi/minikb/internal/tool"
)

const (
	defaultTopK       = 5
	defaultCandidateN = 50
)

// main 是 mini-kb 的命令行入口。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "[错误]", err)
		os.Exit(1)
	}
}

// run 根据第一个参数分发子命令，便于测试时直接调用。
func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		printUsage(out)
		return nil
	}

	switch args[0] {
	case "init":
		return runInit(ctx, args[1:], out)
	case "index":
		return runIndex(ctx, args[1:], out)
	case "search":
		return runSearch(ctx, args[1:], out)
	case "ask":
		return runAsk(ctx, args[1:], out)
	case "agent":
		return runAgent(ctx, args[1:], out)
	case "compare":
		return runCompare(ctx, args[1:], out)
	case "help", "-h", "--help":
		printUsage(out)
		return nil
	default:
		return fmt.Errorf("未知命令 %q，执行 `minikb help` 查看用法", args[0])
	}
}

// runInit 初始化 pgvector 表结构。
func runInit(ctx context.Context, args []string, out io.Writer) error {
	fs := newFlagSet("init", out)
	reset := fs.Bool("reset", false, "删除并重建 kb_chunks 表")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.Load()
	store, err := rag.NewPgVectorStore(ctx, cfg.DSN, cfg.EmbedDim)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Setup(ctx, *reset); err != nil {
		return err
	}
	fmt.Fprintf(out, "数据库初始化完成：dim=%d reset=%v\n", cfg.EmbedDim, *reset)
	return nil
}

// runIndex 遍历文档目录，完成切分、向量化和写入。
func runIndex(ctx context.Context, args []string, out io.Writer) error {
	fs := newFlagSet("index", out)
	docsDir := fs.String("docs", "./docs", "待索引文档目录")
	reset := fs.Bool("reset", false, "索引前删除并重建 kb_chunks 表")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.Load()
	embedder, err := buildEmbedder(cfg)
	if err != nil {
		return err
	}
	store, err := rag.NewPgVectorStore(ctx, cfg.DSN, embedder.Dim())
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Setup(ctx, *reset); err != nil {
		return err
	}
	chunker := rag.NewRecursiveChunker(cfg.ChunkSize, cfg.Overlap)
	stats, err := rag.IndexDir(ctx, *docsDir, chunker, embedder, store)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "索引完成：files=%d chunks=%d dim=%d docs=%s\n", stats.Files, stats.Chunks, embedder.Dim(), *docsDir)
	return nil
}

// runSearch 执行检索，并支持 hybrid、vector、keyword 三种模式。
func runSearch(ctx context.Context, args []string, out io.Writer) error {
	fs := newFlagSet("search", out)
	mode := fs.String("mode", "hybrid", "检索模式：hybrid、vector、keyword")
	topK := fs.Int("top", defaultTopK, "返回片段数量")
	if err := fs.Parse(args); err != nil {
		return err
	}
	query, err := queryFromArgs(fs.Args())
	if err != nil {
		return err
	}

	store, retriever, err := buildRetriever(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	docs, err := retrieveByMode(ctx, retriever, *mode, query, *topK)
	if err != nil {
		return err
	}
	printDocuments(out, docs)
	return nil
}

// runAsk 先检索资料，再生成带来源的回答。
func runAsk(ctx context.Context, args []string, out io.Writer) error {
	fs := newFlagSet("ask", out)
	useLLM := fs.Bool("llm", false, "调用 LLM 生成综合回答")
	topK := fs.Int("top", defaultTopK, "用于回答的资料片段数量")
	if err := fs.Parse(args); err != nil {
		return err
	}
	question, err := queryFromArgs(fs.Args())
	if err != nil {
		return err
	}

	store, retriever, err := buildRetriever(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	docs, err := retriever.Retrieve(ctx, question, *topK)
	if err != nil {
		return err
	}
	var provider llm.Provider
	if *useLLM {
		provider, err = openai.NewFromEnv(false)
		if err != nil {
			return err
		}
	}
	answer, err := qa.AnswerWithSources(ctx, provider, "", question, docs)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, answer)
	return nil
}

// runAgent 把知识库检索包装成工具，由模型按需调用。
func runAgent(ctx context.Context, args []string, out io.Writer) error {
	fs := newFlagSet("agent", out)
	maxSteps := fs.Int("steps", 8, "最大模型调用轮数")
	if err := fs.Parse(args); err != nil {
		return err
	}
	question, err := queryFromArgs(fs.Args())
	if err != nil {
		return err
	}

	store, retriever, err := buildRetriever(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	provider, err := openai.NewFromEnv(true)
	if err != nil {
		return err
	}
	registry := tool.NewRegistry(rag.SearchTool(retriever))
	runner := agent.New(provider, "", registry,
		agent.WithMaxSteps(*maxSteps),
		agent.WithSystemPrompt(agenticRAGPrompt()),
	)

	for event := range runner.RunStream(ctx, question) {
		switch event.Type {
		case agent.EventToolCall:
			fmt.Fprintf(out, "\n[工具调用] %s %s\n", event.Tool, event.Args)
		case agent.EventToolResult:
			fmt.Fprintf(out, "[工具结果]\n%s\n", event.Text)
		case agent.EventAnswerDelta:
			fmt.Fprintln(out, event.Text)
		case agent.EventError:
			return errors.New(event.Text)
		case agent.EventDone:
			fmt.Fprintln(out, "[完成]")
		}
	}
	return nil
}

// runCompare 对比纯向量检索与 hybrid+rerank 的结果。
func runCompare(ctx context.Context, args []string, out io.Writer) error {
	fs := newFlagSet("compare", out)
	topK := fs.Int("top", defaultTopK, "每种模式返回片段数量")
	if err := fs.Parse(args); err != nil {
		return err
	}
	query, err := queryFromArgs(fs.Args())
	if err != nil {
		return err
	}

	store, retriever, err := buildRetriever(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	vectorDocs, err := retriever.VectorOnly(ctx, query, *topK)
	if err != nil {
		return err
	}
	hybridDocs, err := retriever.Retrieve(ctx, query, *topK)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "== Vector Only ==")
	printDocuments(out, vectorDocs)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "== Hybrid + Rerank ==")
	printDocuments(out, hybridDocs)
	return nil
}

// buildRetriever 组装检索管线：Embedder、PgVectorStore、RRF 融合后的 Retriever。
func buildRetriever(ctx context.Context) (*rag.PgVectorStore, *rag.Retriever, error) {
	cfg := config.Load()
	embedder, err := buildEmbedder(cfg)
	if err != nil {
		return nil, nil, err
	}
	store, err := rag.NewPgVectorStore(ctx, cfg.DSN, embedder.Dim())
	if err != nil {
		return nil, nil, err
	}
	if err := store.Setup(ctx, false); err != nil {
		store.Close()
		return nil, nil, err
	}
	return store, &rag.Retriever{
		Store:      store,
		Embedder:   embedder,
		Reranker:   rag.LexicalReranker{},
		CandidateN: defaultCandidateN,
	}, nil
}

// buildEmbedder 根据配置选择本地 embedding 或 OpenAI 兼容 embedding。
func buildEmbedder(cfg config.Config) (rag.Embedder, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Embedder)) {
	case "", "local":
		return embed.NewHashEmbedder(cfg.EmbedDim), nil
	case "openai":
		return openai.NewEmbedderFromEnv()
	default:
		return nil, fmt.Errorf("MINIKB_EMBEDDER=%q 暂不支持，可选值：local、openai", cfg.Embedder)
	}
}

// retrieveByMode 让命令行可以单独验证不同召回路径。
func retrieveByMode(ctx context.Context, retriever *rag.Retriever, mode string, query string, topK int) ([]rag.Document, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "hybrid":
		return retriever.Retrieve(ctx, query, topK)
	case "vector":
		return retriever.VectorOnly(ctx, query, topK)
	case "keyword":
		return retriever.KeywordOnly(ctx, query, topK)
	default:
		return nil, fmt.Errorf("未知检索模式 %q，可选值：hybrid、vector、keyword", mode)
	}
}

// queryFromArgs 把剩余参数合并成问题文本。
func queryFromArgs(args []string) (string, error) {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return "", errors.New("缺少问题或查询文本")
	}
	return query, nil
}

// printDocuments 输出带来源标注的检索结果。
func printDocuments(out io.Writer, docs []rag.Document) {
	fmt.Fprintln(out, rag.FormatDocuments(docs))
}

// agenticRAGPrompt 约束模型通过工具按需检索知识库。
func agenticRAGPrompt() string {
	return strings.TrimSpace(`
你是 mini-kb 知识库助手。

工作规则：
1. 回答课程、产品、发布说明、规范、文档资料相关问题前，先调用 search_knowledge_base。
2. 如果第一次检索结果不足，可以换一个查询词再次调用 search_knowledge_base。
3. 最终回答必须基于工具返回的资料，并使用 [来源 N] 标注依据。
4. 工具没有返回足够资料时，说明“知识库中未找到足够资料”，并给出还需要补充的资料类型。
`)
}

// newFlagSet 创建子命令参数解析器。
func newFlagSet(name string, out io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	return fs
}

// printUsage 输出总用法。
func printUsage(out io.Writer) {
	fmt.Fprint(out, strings.TrimSpace(`
mini-kb：M07 记忆系统与 Agentic RAG 配套练习

用法：
  minikb init [-reset]
      初始化 PostgreSQL + pgvector 表结构。

  minikb index [-docs ./docs] [-reset]
      遍历 .md/.txt 文档，切分、向量化并写入 kb_chunks。

  minikb search [-mode hybrid|vector|keyword] [-top 5] "问题"
      检索知识库并输出来源片段。

  minikb ask [-llm] [-top 5] "问题"
      先检索资料，再生成带来源的回答；未加 -llm 时输出离线摘要。

  minikb agent [-steps 8] "问题"
      把知识库检索作为 Agent 工具，由模型决定检索时机和查询内容。

  minikb compare [-top 5] "问题"
      对比纯向量检索与 hybrid+rerank。
`))
	fmt.Fprintln(out)
}

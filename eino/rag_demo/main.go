package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const defaultQuery = "Aurora 手机的保修期是多久？"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("RAG demo 执行失败", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("eino-rag", flag.ContinueOnError)
	mode := flags.String("mode", "all", "运行模式：index、query 或 all")
	docsDir := flags.String("docs", cfg.DocsDir, "待加载的文档目录或单个文件")
	query := flags.String("query", defaultQuery, "检索问题")
	topK := flags.Int("topk", defaultTopK, "返回的相关文档块数量")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg.DocsDir = *docsDir

	normalizedMode := strings.ToLower(strings.TrimSpace(*mode))
	if normalizedMode != "index" && normalizedMode != "query" && normalizedMode != "all" {
		return fmt.Errorf("未知运行模式 %q，可选值为 index、query、all", *mode)
	}
	if (normalizedMode == "query" || normalizedMode == "all") && strings.TrimSpace(*query) == "" {
		return errors.New("query 模式需要提供非空 -query")
	}
	if *topK <= 0 {
		return errors.New("-topk 必须大于 0")
	}

	embedder, err := newEmbedder(ctx, cfg)
	if err != nil {
		return err
	}

	if normalizedMode == "index" || normalizedMode == "all" {
		docs, err := loadDocuments(ctx, cfg.DocsDir)
		if err != nil {
			return err
		}
		slog.Info("文档加载完成", "documents", len(docs), "path", cfg.DocsDir)

		chunks, err := transformDocuments(ctx, docs, cfg.ChunkSize, cfg.ChunkOverlap)
		if err != nil {
			return err
		}
		slog.Info("文档切分完成", "chunks", len(chunks), "chunk_size", cfg.ChunkSize, "overlap", cfg.ChunkOverlap)

		if err := embedDocuments(ctx, embedder, chunks, cfg.EmbeddingDimension, cfg.EmbeddingBatchSize); err != nil {
			return err
		}
		slog.Info("文档向量化完成", "chunks", len(chunks), "dimension", cfg.EmbeddingDimension)

		ids, err := indexDocuments(ctx, cfg, chunks)
		if err != nil {
			return err
		}
		slog.Info("Milvus 索引写入完成", "collection", cfg.Collection, "documents", len(ids))
	}

	if normalizedMode == "query" || normalizedMode == "all" {
		docs, err := retrieveDocuments(ctx, cfg, embedder, *query, *topK)
		if err != nil {
			return err
		}
		printRetrievedDocuments(*query, docs)
	}
	return nil
}

func printRetrievedDocuments(query string, docs []*schema.Document) {
	fmt.Printf("\n问题：%s\n", query)
	if len(docs) == 0 {
		fmt.Println("没有检索到相关文档。")
		return
	}
	for i, doc := range docs {
		source, _ := doc.MetaData["source"].(string)
		fmt.Printf("\n[%d] score=%.4f id=%s source=%s\n%s\n", i+1, doc.Score(), doc.ID, source, doc.Content)
	}
}

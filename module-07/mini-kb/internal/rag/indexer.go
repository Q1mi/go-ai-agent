package rag

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
)

// IndexStats 描述索引执行结果。
type IndexStats struct {
	Files  int
	Chunks int
}

// IndexDir 遍历目录，把 .md 和 .txt 文档切分、向量化并写入存储。
func IndexDir(ctx context.Context, dir string, chunker *RecursiveChunker, embedder Embedder, store *PgVectorStore) (IndexStats, error) {
	var stats IndexStats
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		if ext := filepath.Ext(path); ext != ".md" && ext != ".txt" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		chunks := chunker.Split(string(data))
		if len(chunks) == 0 {
			return nil
		}
		embs, err := embedder.Embed(ctx, chunks)
		if err != nil {
			return err
		}
		docID, err := filepath.Rel(dir, path)
		if err != nil {
			docID = path
		}
		if err := store.Add(ctx, docID, chunks, embs); err != nil {
			return err
		}
		stats.Files++
		stats.Chunks += len(chunks)
		return nil
	})
	return stats, err
}

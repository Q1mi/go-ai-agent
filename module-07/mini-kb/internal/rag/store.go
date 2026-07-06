package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgVectorStore 使用 PostgreSQL + pgvector 存储和检索文本片段。
type PgVectorStore struct {
	pool *pgxpool.Pool
	dim  int
}

// NewPgVectorStore 连接 PostgreSQL。
func NewPgVectorStore(ctx context.Context, dsn string, dim int) (*PgVectorStore, error) {
	if dim <= 0 || dim > 4096 {
		return nil, fmt.Errorf("无效向量维度: %d", dim)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PgVectorStore{pool: pool, dim: dim}, nil
}

// Close 关闭数据库连接池。
func (store *PgVectorStore) Close() {
	if store != nil && store.pool != nil {
		store.pool.Close()
	}
}

// Setup 创建扩展、表和索引。reset=true 时会重建表。
func (store *PgVectorStore) Setup(ctx context.Context, reset bool) error {
	if _, err := store.pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return err
	}
	if reset {
		if _, err := store.pool.Exec(ctx, `DROP TABLE IF EXISTS kb_chunks`); err != nil {
			return err
		}
	}
	createTable := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS kb_chunks (
	id BIGSERIAL PRIMARY KEY,
	doc_id TEXT NOT NULL,
	chunk_index INTEGER NOT NULL,
	content TEXT NOT NULL,
	embedding vector(%d) NOT NULL,
	search_text tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (doc_id, chunk_index)
)`, store.dim)
	if _, err := store.pool.Exec(ctx, createTable); err != nil {
		return err
	}
	if _, err := store.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS kb_chunks_search_idx ON kb_chunks USING GIN (search_text)`); err != nil {
		return err
	}
	if _, err := store.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS kb_chunks_embedding_hnsw_idx ON kb_chunks USING hnsw (embedding vector_cosine_ops)`); err != nil {
		return err
	}
	return nil
}

// Add 写入一个文档的所有 chunk。相同 docID 会先删除再写入。
func (store *PgVectorStore) Add(ctx context.Context, docID string, chunks []string, embs [][]float32) error {
	if len(chunks) != len(embs) {
		return fmt.Errorf("chunks 与 embeddings 数量不一致")
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM kb_chunks WHERE doc_id = $1`, docID); err != nil {
		return err
	}
	for i := range chunks {
		if len(embs[i]) != store.dim {
			return fmt.Errorf("embedding[%d] 维度=%d，期望=%d", i, len(embs[i]), store.dim)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO kb_chunks (doc_id, chunk_index, content, embedding) VALUES ($1, $2, $3, $4::vector)`,
			docID, i, chunks[i], vecLiteral(embs[i])); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Search 按向量余弦相似度检索。
func (store *PgVectorStore) Search(ctx context.Context, queryEmb []float32, k int) ([]Document, error) {
	if k <= 0 {
		return nil, nil
	}
	rows, err := store.pool.Query(ctx,
		`SELECT id, doc_id, chunk_index, content, 1 - (embedding <=> $1::vector) AS score
		 FROM kb_chunks
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`,
		vecLiteral(queryEmb), k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocuments(rows)
}

// KeywordSearch 使用 PostgreSQL 全文检索，空结果时退回 ILIKE。
func (store *PgVectorStore) KeywordSearch(ctx context.Context, query string, k int) ([]Document, error) {
	query = strings.TrimSpace(query)
	if query == "" || k <= 0 {
		return nil, nil
	}
	rows, err := store.pool.Query(ctx,
		`WITH q AS (SELECT plainto_tsquery('simple', $1) AS query)
		 SELECT id, doc_id, chunk_index, content, ts_rank(search_text, q.query) AS score
		 FROM kb_chunks, q
		 WHERE search_text @@ q.query
		 ORDER BY score DESC
		 LIMIT $2`,
		query, k)
	if err != nil {
		return nil, err
	}
	docs, scanErr := scanDocuments(rows)
	if scanErr != nil {
		return nil, scanErr
	}
	if len(docs) > 0 {
		return docs, nil
	}
	return store.keywordFallback(ctx, query, k)
}

func (store *PgVectorStore) keywordFallback(ctx context.Context, query string, k int) ([]Document, error) {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		terms = []string{query}
	}
	clauses := make([]string, 0, len(terms))
	args := make([]any, 0, len(terms)+1)
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		args = append(args, "%"+term+"%")
		clauses = append(clauses, fmt.Sprintf("content ILIKE $%d", len(args)))
	}
	if len(clauses) == 0 {
		return nil, nil
	}
	args = append(args, k)
	sql := fmt.Sprintf(
		`SELECT id, doc_id, chunk_index, content, 0.5::float8 AS score
		 FROM kb_chunks
		 WHERE %s
		 ORDER BY id
		 LIMIT $%d`,
		strings.Join(clauses, " OR "),
		len(args),
	)
	rows, err := store.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocuments(rows)
}

func scanDocuments(rows pgx.Rows) ([]Document, error) {
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var id int64
		var doc Document
		var score float64
		if err := rows.Scan(&id, &doc.DocID, &doc.ChunkIndex, &doc.Content, &score); err != nil {
			return nil, err
		}
		doc.ID = strconv.FormatInt(id, 10)
		doc.Score = float32(score)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func vecLiteral(v []float32) string {
	parts := make([]string, len(v))
	for i, value := range v {
		parts[i] = fmt.Sprintf("%g", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

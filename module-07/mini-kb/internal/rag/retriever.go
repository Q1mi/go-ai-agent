package rag

import (
	"context"
	"sort"
	"strings"
)

const defaultCandidateN = 50

// Reranker 对粗召回候选做精排。
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []Document, topN int) ([]Document, error)
}

// Retriever 封装向量检索、关键词检索、RRF 融合和 rerank。
type Retriever struct {
	Store      *PgVectorStore
	Embedder   Embedder
	Reranker   Reranker
	CandidateN int
}

// Retrieve 执行混合检索并返回 topN。
func (retriever *Retriever) Retrieve(ctx context.Context, query string, topN int) ([]Document, error) {
	if topN <= 0 {
		topN = 5
	}
	candidateN := retriever.CandidateN
	if candidateN <= 0 {
		candidateN = defaultCandidateN
	}
	if candidateN < topN {
		candidateN = topN
	}
	vecHits, err := retriever.VectorOnly(ctx, query, candidateN)
	if err != nil {
		return nil, err
	}
	kwHits, err := retriever.Store.KeywordSearch(ctx, query, candidateN)
	if err != nil {
		return nil, err
	}
	candidates := fuseCandidates(vecHits, kwHits, candidateN)
	if retriever.Reranker == nil {
		return top(candidates, topN), nil
	}
	return retriever.Reranker.Rerank(ctx, query, candidates, topN)
}

// VectorOnly 只执行向量检索，便于做对比实验。
func (retriever *Retriever) VectorOnly(ctx context.Context, query string, topN int) ([]Document, error) {
	if topN <= 0 {
		topN = 5
	}
	embs, err := retriever.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	return retriever.Store.Search(ctx, embs[0], topN)
}

// KeywordOnly 只执行关键词检索。
func (retriever *Retriever) KeywordOnly(ctx context.Context, query string, topN int) ([]Document, error) {
	return retriever.Store.KeywordSearch(ctx, query, topN)
}

func fuseCandidates(vecHits, kwHits []Document, limit int) []Document {
	byID := make(map[string]Document, len(vecHits)+len(kwHits))
	vecRank := make([]string, 0, len(vecHits))
	kwRank := make([]string, 0, len(kwHits))
	for _, doc := range vecHits {
		vecRank = append(vecRank, doc.ID)
		byID[doc.ID] = doc
	}
	for _, doc := range kwHits {
		kwRank = append(kwRank, doc.ID)
		if old, ok := byID[doc.ID]; ok && old.Score > doc.Score {
			continue
		}
		byID[doc.ID] = doc
	}
	ids := RRF([][]string{vecRank, kwRank}, 60)
	out := make([]Document, 0, min(limit, len(ids)))
	for _, id := range ids {
		if len(out) >= limit {
			break
		}
		out = append(out, byID[id])
	}
	return out
}

func top(docs []Document, n int) []Document {
	if n <= 0 || len(docs) <= n {
		return docs
	}
	return docs[:n]
}

// LexicalReranker 用词项覆盖度做本地重排，适合练习和离线测试。
type LexicalReranker struct{}

// Rerank 对候选按词项命中和粗召回分数排序。
func (reranker LexicalReranker) Rerank(ctx context.Context, query string, docs []Document, topN int) ([]Document, error) {
	terms := uniqueTerms(query)
	for i := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		docs[i].Score = docs[i].Score + float32(termCoverage(terms, docs[i].Content))
	}
	sort.SliceStable(docs, func(i, j int) bool {
		if docs[i].Score == docs[j].Score {
			return docs[i].ID < docs[j].ID
		}
		return docs[i].Score > docs[j].Score
	})
	return top(docs, topN), nil
}

func uniqueTerms(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range strings.Fields(strings.ToLower(text)) {
		term = strings.Trim(term, " \t\n\r，。！？、:：;；,.()[]{}<>「」“”\"'")
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			term := string(r)
			if !seen[term] {
				seen[term] = true
				out = append(out, term)
			}
		}
	}
	return out
}

func termCoverage(terms []string, content string) float64 {
	if len(terms) == 0 {
		return 0
	}
	content = strings.ToLower(content)
	hit := 0
	for _, term := range terms {
		if strings.Contains(content, term) {
			hit++
		}
	}
	return float64(hit) / float64(len(terms))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

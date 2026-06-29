package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Document 表示知识库中的一篇本地文档。
type Document struct {
	ID      string
	Title   string
	Source  string
	Content string
}

// ScoredDocument 表示一次检索命中的文档和相关性分数。
type ScoredDocument struct {
	Document Document
	Score    int
}

// LoadDir 读取目录中的 .md 和 .txt 文档，并转换为知识库文档列表。
func LoadDir(dir string) ([]Document, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("知识库目录不能为空")
	}
	var docs []Document
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.TrimSpace(string(raw))
		if content == "" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		docs = append(docs, Document{
			ID:      fmt.Sprintf("D%d", len(docs)+1),
			Title:   titleFromContent(content, rel),
			Source:  filepath.ToSlash(rel),
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("知识库目录 %s 中没有 .md 或 .txt 文档", dir)
	}
	return docs, nil
}

// Retrieve 使用简单词项匹配从知识库中检索 top-k 文档。
//
// 这个实现方便学员理解“检索资料进入上下文”的主流程，后续课程可以替换成
// embedding、向量库和 rerank。
func Retrieve(query string, docs []Document, topK int) []ScoredDocument {
	if topK <= 0 {
		topK = 3
	}
	queryTokens := tokenize(query)
	scored := make([]ScoredDocument, 0, len(docs))
	for _, doc := range docs {
		score := scoreDocument(queryTokens, doc)
		if score > 0 {
			scored = append(scored, ScoredDocument{Document: doc, Score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Document.ID < scored[j].Document.ID
		}
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > topK {
		scored = scored[:topK]
	}
	if len(scored) == 0 {
		for i, doc := range docs {
			if i >= topK {
				break
			}
			scored = append(scored, ScoredDocument{Document: doc})
		}
	}
	return scored
}

// Documents 从带分数的检索结果中提取原始文档列表。
func Documents(scored []ScoredDocument) []Document {
	docs := make([]Document, 0, len(scored))
	for _, item := range scored {
		docs = append(docs, item.Document)
	}
	return docs
}

// titleFromContent 优先使用文档首个非空行作为标题。
func titleFromContent(content string, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return strings.TrimSuffix(filepath.Base(fallback), filepath.Ext(fallback))
}

// scoreDocument 根据查询词在标题和正文中的命中情况计算分数。
func scoreDocument(queryTokens map[string]int, doc Document) int {
	if len(queryTokens) == 0 {
		return 0
	}
	titleTokens := tokenize(doc.Title)
	bodyTokens := tokenize(doc.Content)
	score := 0
	for token, weight := range queryTokens {
		if token == "" {
			continue
		}
		score += titleTokens[token] * weight * 4
		score += bodyTokens[token] * weight
	}
	return score
}

// tokenize 把中英文混合文本切成简单词项。
func tokenize(text string) map[string]int {
	out := make(map[string]int)
	var ascii strings.Builder
	flushASCII := func() {
		if ascii.Len() == 0 {
			return
		}
		token := strings.ToLower(ascii.String())
		if len(token) > 1 {
			out[token]++
		}
		ascii.Reset()
	}
	for _, r := range text {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'):
			ascii.WriteRune(r)
		case unicode.Is(unicode.Han, r):
			flushASCII()
			out[string(r)]++
		default:
			flushASCII()
		}
	}
	flushASCII()
	return out
}

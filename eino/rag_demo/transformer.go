package main

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino/schema"
)

// transformDocuments 先按 Markdown 标题切分，再将超长章节切成带重叠的文本块。
func transformDocuments(ctx context.Context, docs []*schema.Document, chunkSize, overlap int) ([]*schema.Document, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize 必须大于 0")
	}
	if overlap < 0 || overlap >= chunkSize {
		return nil, fmt.Errorf("overlap 必须位于 [0, chunkSize) 区间")
	}

	headerSplitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":    "h1",
			"##":   "h2",
			"###":  "h3",
			"####": "h4",
		},
		TrimHeaders: false,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 Markdown 切分器: %w", err)
	}

	result := make([]*schema.Document, 0, len(docs))
	for _, original := range docs {
		if original == nil {
			continue
		}
		sections, err := headerSplitter.Transform(ctx, []*schema.Document{original})
		if err != nil {
			return nil, fmt.Errorf("按标题切分文档 %q: %w", original.ID, err)
		}

		chunkIndex := 0
		for sectionIndex, section := range sections {
			chunks := splitTextWithOverlap(section.Content, chunkSize, overlap)
			for _, chunk := range chunks {
				content := strings.TrimSpace(chunk.text)
				if content == "" {
					continue
				}

				meta := cloneMetadata(section.MetaData)
				meta["parent_document_id"] = original.ID
				meta["section_index"] = sectionIndex
				meta["chunk_index"] = chunkIndex
				meta["chunk_start_rune"] = chunk.start
				meta["chunk_end_rune"] = chunk.end

				result = append(result, &schema.Document{
					ID:       fmt.Sprintf("%s-chunk-%04d", original.ID, chunkIndex),
					Content:  content,
					MetaData: meta,
				})
				chunkIndex++
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("切分后没有可用文本块")
	}
	return result, nil
}

type textChunk struct {
	text       string
	start, end int
}

func splitTextWithOverlap(text string, chunkSize, overlap int) []textChunk {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}

	chunks := make([]textChunk, 0, (len(runes)+chunkSize-1)/chunkSize)
	for start := 0; start < len(runes); {
		end := min(start+chunkSize, len(runes))
		if end < len(runes) {
			end = nearestBoundary(runes, start, end)
		}
		if end <= start {
			end = min(start+chunkSize, len(runes))
		}

		chunks = append(chunks, textChunk{
			text:  string(runes[start:end]),
			start: start,
			end:   end,
		})
		if end == len(runes) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}

func nearestBoundary(runes []rune, start, desiredEnd int) int {
	// 只在块后半段寻找边界，避免为了一个过早的标点生成很小的文本块。
	lowerBound := start + (desiredEnd-start)/2
	for i := desiredEnd; i > lowerBound; i-- {
		if isTextBoundary(runes[i-1]) {
			return i
		}
	}
	return desiredEnd
}

func isTextBoundary(r rune) bool {
	return r == '\n' || r == '。' || r == '！' || r == '？' || r == '；' ||
		r == '.' || r == '!' || r == '?' || r == ';' || unicode.IsSpace(r)
}

func cloneMetadata(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+6)
	for key, value := range input {
		output[key] = value
	}
	return output
}

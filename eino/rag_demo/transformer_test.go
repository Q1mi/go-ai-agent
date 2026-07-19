package main

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestTransformDocumentsCreatesUniqueChunks(t *testing.T) {
	doc := &schema.Document{
		ID:      "doc-test",
		Content: "# 标题\n\n## 第一节\n\n" + strings.Repeat("内容。", 80) + "\n\n## 第二节\n\n结尾。",
		MetaData: map[string]any{
			"source": "test.md",
		},
	}

	chunks, err := transformDocuments(context.Background(), []*schema.Document{doc}, 80, 10)
	if err != nil {
		t.Fatalf("transformDocuments() error = %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("transformDocuments() returned %d chunks, want at least 3", len(chunks))
	}

	ids := make(map[string]struct{}, len(chunks))
	for i, chunk := range chunks {
		if _, exists := ids[chunk.ID]; exists {
			t.Fatalf("duplicate chunk ID %q", chunk.ID)
		}
		ids[chunk.ID] = struct{}{}
		if chunk.MetaData["source"] != "test.md" {
			t.Fatalf("chunk %d did not preserve source metadata", i)
		}
		if chunk.MetaData["chunk_index"] != i {
			t.Fatalf("chunk %d has chunk_index %v", i, chunk.MetaData["chunk_index"])
		}
	}
}

func TestSplitTextWithOverlap(t *testing.T) {
	chunks := splitTextWithOverlap(strings.Repeat("a", 25), 10, 2)
	if len(chunks) != 3 {
		t.Fatalf("splitTextWithOverlap() returned %d chunks, want 3", len(chunks))
	}
	if chunks[1].start != 8 || chunks[2].start != 16 {
		t.Fatalf("unexpected chunk starts: %d, %d", chunks[1].start, chunks[2].start)
	}
}

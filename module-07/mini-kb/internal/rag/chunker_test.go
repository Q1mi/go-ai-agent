package rag

import (
	"strings"
	"testing"
)

func TestRecursiveChunkerSplit(t *testing.T) {
	tests := []struct {
		name       string
		chunkSize  int
		overlap    int
		text       string
		wantChunks int
		maxLen     int
	}{
		{
			name:       "split by paragraph boundary",
			chunkSize:  12,
			overlap:    0,
			text:       "第一段说明。\n\n第二段说明。\n\n第三段说明。",
			wantChunks: 3,
			maxLen:     12,
		},
		{
			name:       "hard split without separators",
			chunkSize:  10,
			overlap:    0,
			text:       strings.Repeat("一", 25),
			wantChunks: 3,
			maxLen:     10,
		},
		{
			name:       "trim empty input",
			chunkSize:  10,
			overlap:    0,
			text:       " \n\t ",
			wantChunks: 0,
			maxLen:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewRecursiveChunker(tt.chunkSize, tt.overlap)
			got := chunker.Split(tt.text)
			if len(got) != tt.wantChunks {
				t.Fatalf("len(Split())=%d, want %d, chunks=%q", len(got), tt.wantChunks, got)
			}
			for _, chunk := range got {
				if runeLen(chunk) > tt.maxLen {
					t.Fatalf("chunk length=%d, want <=%d, chunk=%q", runeLen(chunk), tt.maxLen, chunk)
				}
				if strings.TrimSpace(chunk) != chunk {
					t.Fatalf("chunk should be trimmed: %q", chunk)
				}
			}
		})
	}
}

func TestRecursiveChunkerSplitWithOverlap(t *testing.T) {
	chunker := NewRecursiveChunker(10, 3)
	got := chunker.Split("abcdefghij klmnopqrst uvwxyz")
	if len(got) != 3 {
		t.Fatalf("len(Split())=%d, want 3, chunks=%q", len(got), got)
	}
	if got[0] != "abcdefghij" {
		t.Fatalf("first chunk=%q, want %q", got[0], "abcdefghij")
	}
	if !strings.HasPrefix(got[1], "hij") {
		t.Fatalf("second chunk should start with overlap tail %q, got %q", "hij", got[1])
	}
	if !strings.HasPrefix(got[2], "rst") {
		t.Fatalf("third chunk should start with overlap tail %q, got %q", "rst", got[2])
	}
}

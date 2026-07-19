package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDocuments(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n\nalpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := loadDocuments(context.Background(), dir)
	if err != nil {
		t.Fatalf("loadDocuments() error = %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("loadDocuments() returned %d docs, want 2", len(docs))
	}
	if docs[0].ID == "" || docs[1].ID == "" || docs[0].ID == docs[1].ID {
		t.Fatalf("document IDs must be unique: %q, %q", docs[0].ID, docs[1].ID)
	}
	for _, doc := range docs {
		if doc.MetaData["source"] == "" {
			t.Fatalf("document %q has no source metadata", doc.ID)
		}
	}
}

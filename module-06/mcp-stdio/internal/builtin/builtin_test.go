package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCalcTool(t *testing.T) {
	out, err := NewCalcTool().Call(context.Background(), json.RawMessage(`{"expr":"1+2*3"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out != "1+2*3 = 7" {
		t.Fatalf("out = %q", out)
	}
}

func TestTimeTool(t *testing.T) {
	fixed := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	out, err := NewTimeTool(func() time.Time { return fixed }).Call(context.Background(), json.RawMessage(`{"timezone":"Asia/Shanghai"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026-06-29T18:00:00") {
		t.Fatalf("out = %q", out)
	}
}

func TestFileSystemSafePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := NewFileSystem(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.SafePath("../secret.txt"); err == nil {
		t.Fatal("expected path jail error")
	}
	out, err := fs.ReadFileTool().Call(context.Background(), json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Fatalf("out = %q", out)
	}
}

func TestValidateSelectOnly(t *testing.T) {
	if err := ValidateSelectOnly("select * from users limit 10"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSelectOnly("delete from users"); err == nil {
		t.Fatal("expected delete to be rejected")
	}
}

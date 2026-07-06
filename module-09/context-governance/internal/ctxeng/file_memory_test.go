package ctxeng

import (
	"strings"
	"testing"
)

func TestFileMemoryOffloadAndRead(t *testing.T) {
	memory := &FileMemory{Dir: t.TempDir()}
	content := strings.Repeat("这是一段很长的工具结果。", 80)
	placeholder, info, err := memory.OffloadWithInfo(content, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Offloaded {
		t.Fatalf("expected offloaded content")
	}
	if !strings.Contains(placeholder, info.ID) {
		t.Fatalf("placeholder should contain id, got %q", placeholder)
	}
	got, err := memory.Read(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Fatalf("read content mismatch")
	}
}

func TestFileMemorySmallContent(t *testing.T) {
	memory := &FileMemory{Dir: t.TempDir()}
	content := "短结果"
	got, info, err := memory.OffloadWithInfo(content, 100)
	if err != nil {
		t.Fatal(err)
	}
	if info.Offloaded {
		t.Fatalf("small content should stay inline")
	}
	if got != content {
		t.Fatalf("got %q, want %q", got, content)
	}
}

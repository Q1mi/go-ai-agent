package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirAndRetrieve(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "timeout.md"), []byte("# 超时配置\n请求超时默认 30 秒。"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deploy.txt"), []byte("部署配置支持 YAML 和环境变量。"), 0o600); err != nil {
		t.Fatal(err)
	}
	docs, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("len(docs) = %d", len(docs))
	}
	hits := Retrieve("如何设置超时", docs, 1)
	if len(hits) != 1 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Document.Source != "timeout.md" {
		t.Fatalf("top hit = %+v", hits[0])
	}
}

func TestLoadDirEmpty(t *testing.T) {
	_, err := LoadDir(t.TempDir())
	if err == nil {
		t.Fatal("期望空知识库错误")
	}
}

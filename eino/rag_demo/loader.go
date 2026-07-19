package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// loadDocuments 递归加载 docsDir 中的文档，并为每个原始文档生成稳定 ID。
func loadDocuments(ctx context.Context, docsDir string) ([]*schema.Document, error) {
	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{})
	if err != nil {
		return nil, fmt.Errorf("初始化文件加载器: %w", err)
	}

	paths, err := documentPaths(docsDir)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("目录 %q 中没有可加载的文档", docsDir)
	}

	docs := make([]*schema.Document, 0, len(paths))
	for _, path := range paths {
		loaded, err := loader.Load(ctx, document.Source{URI: path})
		if err != nil {
			return nil, fmt.Errorf("加载文档 %q: %w", path, err)
		}
		for i, doc := range loaded {
			if doc == nil || strings.TrimSpace(doc.Content) == "" {
				continue
			}
			doc.ID = sourceDocumentID(path, i)
			if doc.MetaData == nil {
				doc.MetaData = make(map[string]any)
			}
			doc.MetaData["source"] = path
			doc.MetaData["source_document_index"] = i
			docs = append(docs, doc)
		}
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("目录 %q 中的文档内容均为空", docsDir)
	}
	return docs, nil
}

func documentPaths(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("访问文档路径 %q: %w", root, err)
	}
	if !info.IsDir() {
		return []string{root}, nil
	}

	var paths []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历文档目录 %q: %w", root, err)
	}
	sort.Strings(paths)
	return paths, nil
}

func sourceDocumentID(path string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", filepath.Clean(path), index)))
	return fmt.Sprintf("doc-%x", sum[:10])
}

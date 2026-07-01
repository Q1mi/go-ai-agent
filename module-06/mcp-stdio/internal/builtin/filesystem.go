package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/q1mi/mcptools/internal/tool"
)

// FileSystem 把文件访问限制在 root 目录内。
type FileSystem struct {
	root string
}

// NewFileSystem 创建带路径围栏的文件系统工具集合。
func NewFileSystem(root string) (*FileSystem, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &FileSystem{root: abs}, nil
}

// SafePath 把相对路径解析为 root 内的绝对路径，越界时返回错误。
func (fs *FileSystem) SafePath(path string) (string, error) {
	clean := filepath.Clean(filepath.Join(fs.root, path))
	if clean != fs.root && !strings.HasPrefix(clean, fs.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界，拒绝访问: %s", path)
	}
	return clean, nil
}

type readFileArgs struct {
	Path string `json:"path" desc:"相对于根目录的文件路径"`
}

// ReadFileTool 创建受限文件读取工具。
func (fs *FileSystem) ReadFileTool() tool.Tool {
	return tool.NewTypedTool("read_file", "读取根目录下的文本文件内容", func(_ context.Context, args readFileArgs) (string, error) {
		path, err := fs.SafePath(args.Path)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("读取失败: %w", err)
		}
		const maxBytes = 100 * 1024
		if len(data) > maxBytes {
			data = data[:maxBytes]
		}
		return string(data), nil
	})
}

package ctxeng

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileMemory 把大块内容外置到磁盘，上下文中只保留引用和摘要。
type FileMemory struct {
	Dir string
}

// OffloadInfo 记录一次外置动作。
type OffloadInfo struct {
	ID           string
	Offloaded    bool
	BeforeTokens int
	AfterTokens  int
	Placeholder  string
}

// Offload 内容超过阈值时写入磁盘，返回适合放入上下文的占位文本。
func (memory *FileMemory) Offload(content string, threshold int) (string, error) {
	placeholder, _, err := memory.OffloadWithInfo(content, threshold)
	return placeholder, err
}

// OffloadWithInfo 执行外置并返回统计信息。
func (memory *FileMemory) OffloadWithInfo(content string, threshold int) (string, OffloadInfo, error) {
	info := OffloadInfo{BeforeTokens: EstimateTokens(content)}
	if threshold <= 0 || info.BeforeTokens <= threshold {
		info.AfterTokens = info.BeforeTokens
		info.Placeholder = content
		return content, info, nil
	}
	if strings.TrimSpace(memory.Dir) == "" {
		return "", info, fmt.Errorf("FileMemory.Dir 不能为空")
	}
	if err := os.MkdirAll(memory.Dir, 0o700); err != nil {
		return "", info, err
	}
	id := memory.newID(content)
	path := memory.path(id)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", info, err
	}
	head := []rune(content)
	if len(head) > 180 {
		head = head[:180]
	}
	placeholder := fmt.Sprintf("[内容已外置 id=%s，摘要:%s…（需全文用 read_memory 读取）]", id, strings.TrimSpace(string(head)))
	info.ID = id
	info.Offloaded = true
	info.Placeholder = placeholder
	info.AfterTokens = EstimateTokens(placeholder)
	return placeholder, info, nil
}

// Read 根据 id 读取外置内容。
func (memory *FileMemory) Read(id string) (string, error) {
	if strings.TrimSpace(memory.Dir) == "" {
		return "", fmt.Errorf("FileMemory.Dir 不能为空")
	}
	data, err := os.ReadFile(memory.path(id))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (memory *FileMemory) newID(content string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%d:%s", time.Now().UnixNano(), content)))
	return "mem-" + hex.EncodeToString(sum[:])[:16]
}

func (memory *FileMemory) path(id string) string {
	base := filepath.Base(strings.TrimSpace(id))
	base = strings.TrimSuffix(base, ".txt")
	return filepath.Join(memory.Dir, base+".txt")
}

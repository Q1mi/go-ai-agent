package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store 是 Agent 状态持久化接口。
type Store interface {
	// Save 保存指定 session 的状态快照。
	Save(ctx context.Context, sessionID string, state *State) error
	// Load 读取指定 session 的状态快照。
	Load(ctx context.Context, sessionID string) (*State, error)
}

// FileStore 对应课件 4.10 的最小文件持久化实现。
// 它适合本地练习和调试；生产环境可替换成数据库或对象存储。
type FileStore struct {
	dir string
}

// NewFileStore 创建基于本地目录的状态存储。
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

// Save 把状态保存为 JSON 文件。
func (store *FileStore) Save(_ context.Context, sessionID string, state *State) error {
	if store == nil {
		return fmt.Errorf("FileStore 未初始化")
	}
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(store.path(sessionID), raw, 0o600)
}

// Load 从 JSON 文件恢复状态。
func (store *FileStore) Load(_ context.Context, sessionID string) (*State, error) {
	if store == nil {
		return nil, fmt.Errorf("FileStore 未初始化")
	}
	raw, err := os.ReadFile(store.path(sessionID))
	if err != nil {
		return nil, fmt.Errorf("加载会话 %s 失败: %w", sessionID, err)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.ActionCounts == nil {
		state.ActionCounts = make(map[string]int)
	}
	return &state, nil
}

// path 将 sessionID 映射为当前存储目录下的安全文件名。
func (store *FileStore) path(sessionID string) string {
	name := filepath.Base(sessionID)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "default"
	}
	return filepath.Join(store.dir, name+".json")
}

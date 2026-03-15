package offset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store 负责管理每个文件的已提交 offset（committedOffset），并持久化到本地 JSON 文件。
type Store struct {
	mu     sync.RWMutex
	path   string
	offset map[string]int64
}

// NewStore 创建一个新的 offset Store，path 为相对或绝对路径。
func NewStore(path string) *Store {
	return &Store{
		path:   path,
		offset: make(map[string]int64),
	}
}

// Load 从磁盘加载已存在的 offset 文件，若不存在则忽略。
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	m := make(map[string]int64)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	s.offset = m
	return nil
}

// Get 获取指定文件已提交的 offset。
func (s *Store) Get(path string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.offset[path]
	return v, ok
}

// Set 将指定文件的 committedOffset 更新到给定值（仅在上报成功后调用）。
func (s *Store) Set(path string, off int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offset[path] = off
}

// Delete 删除指定文件的记录（例如文件永久删除时）。
func (s *Store) Delete(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.offset, path)
}

// Save 将当前 offset map 持久化到 JSON 文件。
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.offset, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}


package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MockFileSystem is a mock implementation of FileSystem for testing.
type MockFileSystem struct {
	mu           sync.RWMutex
	Files        map[string][]byte
	Dirs         map[string]bool
	WD           string
	HomeDir      string
	Env          map[string]string
	ExecPath     string
	ExecErr      error
	StatErr      error
	StatCalls    []string
	ReadFileErr  error
	MkdirAllErr  error
	WriteFileErr error
	RemoveAllErr error
	AbsErr       error
	HomeDirErr   error
	TempDirValue string
	Perms        map[string]os.FileMode
}

func (m *MockFileSystem) Getwd() (string, error) {
	return m.WD, nil
}

type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() any           { return nil }

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	m.mu.Lock()
	m.StatCalls = append(m.StatCalls, name)
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.StatErr != nil {
		return nil, m.StatErr
	}
	baseName := filepath.Base(name)
	if _, ok := m.Files[name]; ok {
		return &mockFileInfo{name: baseName, isDir: false}, nil
	}
	if m.Dirs[name] {
		return &mockFileInfo{name: baseName, isDir: true}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ReadFileErr != nil {
		return nil, m.ReadFileErr
	}
	if data, ok := m.Files[name]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) UserHomeDir() (string, error) {
	if m.HomeDirErr != nil {
		return "", m.HomeDirErr
	}
	return m.HomeDir, nil
}

func (m *MockFileSystem) Executable() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ExecErr != nil {
		return "", m.ExecErr
	}
	return m.ExecPath, nil
}

func (m *MockFileSystem) Getenv(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Env == nil {
		return ""
	}
	return m.Env[key]
}

func (m *MockFileSystem) LookupEnv(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Env == nil {
		return "", false
	}
	val, ok := m.Env[key]
	return val, ok
}

func (m *MockFileSystem) TempDir() string {
	if m.TempDirValue != "" {
		return m.TempDirValue
	}
	return "/tmp"
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.MkdirAllErr != nil {
		return m.MkdirAllErr
	}
	if m.Dirs == nil {
		m.Dirs = make(map[string]bool)
	}
	if m.Perms == nil {
		m.Perms = make(map[string]os.FileMode)
	}
	m.Dirs[path] = true
	m.Perms[path] = perm
	return nil
}

func (m *MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.WriteFileErr != nil {
		return m.WriteFileErr
	}
	if m.Files == nil {
		m.Files = make(map[string][]byte)
	}
	if m.Perms == nil {
		m.Perms = make(map[string]os.FileMode)
	}
	m.Files[filename] = data
	m.Perms[filename] = perm
	return nil
}

func (m *MockFileSystem) RemoveAll(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.RemoveAllErr != nil {
		return m.RemoveAllErr
	}
	if path == "" {
		return nil
	}
	for d := range m.Dirs {
		if d == path || (strings.HasPrefix(d, path) && len(d) > len(path) && (d[len(path)] == '/' || d[len(path)] == '\\')) {
			delete(m.Dirs, d)
		}
	}
	for f := range m.Files {
		if f == path || (strings.HasPrefix(f, path) && len(f) > len(path) && (f[len(path)] == '/' || f[len(path)] == '\\')) {
			delete(m.Files, f)
		}
	}
	return nil
}

func (m *MockFileSystem) Abs(path string) (string, error) {
	if m.AbsErr != nil {
		return "", m.AbsErr
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(m.WD, path), nil
}

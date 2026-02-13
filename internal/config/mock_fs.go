package config

import (
	"os"
)

// MockFileSystem is a mock implementation of FileSystem for testing.
type MockFileSystem struct {
	Files        map[string][]byte
	Dirs         map[string]bool
	WD           string
	HomeDir      string
	Env          map[string]string
	ExecPath     string
	ExecErr      error
	StatErr      error
	ReadFileErr  error
	MkdirAllErr  error
	WriteFileErr error
	RemoveAllErr error
	TempDirValue string
}

func (m *MockFileSystem) Getwd() (string, error) {
	return m.WD, nil
}

type mockFileInfo struct {
	os.FileInfo
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.StatErr != nil {
		return nil, m.StatErr
	}
	if _, ok := m.Files[name]; ok {
		return &mockFileInfo{}, nil
	}
	if m.Dirs[name] {
		return &mockFileInfo{}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	if m.ReadFileErr != nil {
		return nil, m.ReadFileErr
	}
	if data, ok := m.Files[name]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) UserHomeDir() (string, error) {
	return m.HomeDir, nil
}

func (m *MockFileSystem) Executable() (string, error) {
	if m.ExecErr != nil {
		return "", m.ExecErr
	}
	return m.ExecPath, nil
}

func (m *MockFileSystem) Getenv(key string) string {
	if m.Env == nil {
		return ""
	}
	return m.Env[key]
}

func (m *MockFileSystem) TempDir() string {
	if m.TempDirValue != "" {
		return m.TempDirValue
	}
	return "/tmp"
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if m.MkdirAllErr != nil {
		return m.MkdirAllErr
	}
	if m.Dirs == nil {
		m.Dirs = make(map[string]bool)
	}
	m.Dirs[path] = true
	return nil
}

func (m *MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if m.WriteFileErr != nil {
		return m.WriteFileErr
	}
	if m.Files == nil {
		m.Files = make(map[string][]byte)
	}
	m.Files[filename] = data
	return nil
}

func (m *MockFileSystem) RemoveAll(path string) error {
	if m.RemoveAllErr != nil {
		return m.RemoveAllErr
	}
	for d := range m.Dirs {
		if d == path || (len(d) > len(path) && d[:len(path)] == path && (d[len(path)] == '/' || d[len(path)] == '\\')) {
			delete(m.Dirs, d)
		}
	}
	for f := range m.Files {
		if f == path || (len(f) > len(path) && f[:len(path)] == path && (f[len(path)] == '/' || f[len(path)] == '\\')) {
			delete(m.Files, f)
		}
	}
	return nil
}

package command

import (
	"context"
	"os"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

type TerminationMockRuntime struct {
	*runtime.MockRuntime
	IsRunning  bool
	InspectErr error
}

func (m *TerminationMockRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	return m.IsRunning, m.ExitCode, m.InspectErr
}

type RootErrorFS struct {
	WriteFileFunc func(path string, data []byte, perm os.FileMode) error
	*config.MockFileSystem
	MkdirErr error
	WriteErr error
}

func (fs *RootErrorFS) MkdirAll(path string, perm os.FileMode) error {
	if fs.MkdirErr != nil {
		return fs.MkdirErr
	}
	return fs.MockFileSystem.MkdirAll(path, perm)
}

func (fs *RootErrorFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if fs.WriteFileFunc != nil {
		return fs.WriteFileFunc(path, data, perm)
	}
	if fs.WriteErr != nil {
		return fs.WriteErr
	}
	return fs.MockFileSystem.WriteFile(path, data, perm)
}

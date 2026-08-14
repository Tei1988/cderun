package command

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// prefetchMockRuntime implements only PullImage to check pulled images and can simulate failures.
type prefetchMockRuntime struct {
	*runtime.MockRuntime
	PulledImages    []string
	PullPolicies    []string
	PullMaxRetries  []int
	PullBackoffBases []time.Duration
	FailForImage    string
}

func (m *prefetchMockRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if m.FailForImage == img {
		return errors.New("simulated pull failure")
	}
	m.PulledImages = append(m.PulledImages, img)
	m.PullPolicies = append(m.PullPolicies, pullPolicy)
	m.PullMaxRetries = append(m.PullMaxRetries, maxRetries)
	m.PullBackoffBases = append(m.PullBackoffBases, backoffBase)
	return nil
}

func (m *prefetchMockRuntime) Close() error {
	return nil
}

func TestUnit_Command_Prefetch_All(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte(`
go:
  image: golang:1.22-alpine
node:
  image: node:20-slim
`),
		},
	}

	mockRuntime := &prefetchMockRuntime{MockRuntime: &runtime.MockRuntime{}}
	args := []string{
		"cderun",
		"--prefetch-all",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)
	assert.Len(t, mockRuntime.PulledImages, 2)
	assert.Contains(t, mockRuntime.PulledImages, "golang:1.22-alpine")
	assert.Contains(t, mockRuntime.PulledImages, "node:20-slim")
	assert.Contains(t, mockRuntime.PullPolicies, "missing") // default policy
}

func TestUnit_Command_Prefetch_Specific(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte(`
go:
  image: golang:1.22-alpine
node:
  image: node:20-slim
python:
  image: python:3.11-alpine
`),
		},
	}

	mockRuntime := &prefetchMockRuntime{MockRuntime: &runtime.MockRuntime{}}
	args := []string{
		"cderun",
		"--prefetch", "node,python",
		"--pull", "always",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)
	assert.Len(t, mockRuntime.PulledImages, 2)
	assert.Contains(t, mockRuntime.PulledImages, "node:20-slim")
	assert.Contains(t, mockRuntime.PulledImages, "python:3.11-alpine")
	assert.NotContains(t, mockRuntime.PulledImages, "golang:1.22-alpine")
	assert.Contains(t, mockRuntime.PullPolicies, "always") // explicitly specified policy
}

func TestUnit_Command_Prefetch_TemplateResolution(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte(`
go:
  image: "golang:{{env:GO_VERSION}}-alpine"
`),
		},
		Env: map[string]string{
			"GO_VERSION": "1.23",
		},
	}

	mockRuntime := &prefetchMockRuntime{MockRuntime: &runtime.MockRuntime{}}
	args := []string{
		"cderun",
		"--prefetch", "go",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)
	assert.Len(t, mockRuntime.PulledImages, 1)
	assert.Equal(t, "golang:1.23-alpine", mockRuntime.PulledImages[0])
}

func TestUnit_Command_Prefetch_Failure(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte(`
go:
  image: golang:1.22-alpine
`),
		},
	}

	mockRuntime := &prefetchMockRuntime{
		MockRuntime:  &runtime.MockRuntime{},
		FailForImage: "golang:1.22-alpine",
	}
	args := []string{
		"cderun",
		"--prefetch", "go",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated pull failure")
}

func TestUnit_Command_Prefetch_InvalidTool(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte(`
go:
  image: golang:1.22-alpine
`),
		},
	}

	mockRuntime := &prefetchMockRuntime{MockRuntime: &runtime.MockRuntime{}}
	args := []string{
		"cderun",
		"--prefetch", "ruby",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `tool "ruby" is not defined`)
}

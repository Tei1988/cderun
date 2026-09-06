package command

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestUnit_Prefetch_DetermineToolsToPrefetch(t *testing.T) {
	toolsCfg := config.ToolsConfig{
		"node":   config.ToolConfig{Image: "node:20-alpine"},
		"python": config.ToolConfig{Image: "python:3.11-slim"},
		"go":     config.ToolConfig{Image: "golang:1.22-alpine"},
	}

	t.Run("PrefetchAll option", func(t *testing.T) {
		res := &config.ResolvedConfig{
			PrefetchAll: true,
		}
		tools, err := determineToolsToPrefetch(res, toolsCfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "node", "python"}, tools)
	})

	t.Run("Comma-separated Prefetch option", func(t *testing.T) {
		res := &config.ResolvedConfig{
			Prefetch: "python, go",
		}
		tools, err := determineToolsToPrefetch(res, toolsCfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "python"}, tools)
	})

	t.Run("Empty prefetch option returns error", func(t *testing.T) {
		res := &config.ResolvedConfig{}
		tools, err := determineToolsToPrefetch(res, toolsCfg)
		require.Error(t, err)
		assert.Nil(t, tools)
		assert.Contains(t, err.Error(), "no tools specified for prefetch")
	})
}

func TestUnit_Prefetch_ResolvePrefetchImages(t *testing.T) {
	mockFS := &config.MockFileSystem{
		Env: map[string]string{"MOCK_IMG_TAG": "alpine"},
	}
	r, err := config.NewExpressionResolverWithFS(nil, mockFS)
	require.NoError(t, err)

	toolsCfg := config.ToolsConfig{
		"node":   config.ToolConfig{Image: "node:{{env:MOCK_IMG_TAG}}"},
		"noimg":  config.ToolConfig{Image: ""},
		"python": config.ToolConfig{Image: "python:3.11"},
	}

	t.Run("Successful expression resolution", func(t *testing.T) {
		imgs, err := resolvePrefetchImages(r, []string{"node", "python"}, toolsCfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"node:alpine", "python:3.11"}, imgs)
	})

	t.Run("Undefined tool in tools configuration", func(t *testing.T) {
		_, err := resolvePrefetchImages(r, []string{"nonexistent"}, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `tool "nonexistent" is not defined`)
	})

	t.Run("Tool without image configured", func(t *testing.T) {
		_, err := resolvePrefetchImages(r, []string{"noimg"}, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `tool "noimg" does not have an image configured`)
	})
}

func TestUnit_Prefetch_PullPrefetchImages(t *testing.T) {
	opts := defaultOptions()
	mockRt := runtime.NewMockRuntime()
	res := &config.ResolvedConfig{
		Pull:            "always",
		PullMaxRetries:  1,
		PullBackoffBase: 1 * time.Millisecond,
	}

	t.Run("Successful image pulls", func(t *testing.T) {
		err := opts.pullPrefetchImages(context.Background(), mockRt, []string{"alpine:latest", "ubuntu:22.04"}, res)
		require.NoError(t, err)
	})

	t.Run("Error on image pull failure", func(t *testing.T) {
		failingRt := &runtime.MockRuntime{
			PullErr: fmt.Errorf("network error"),
		}
		err := opts.pullPrefetchImages(context.Background(), failingRt, []string{"alpine:latest"}, res)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to prefetch image alpine:latest")
	})
}

func TestUnit_Execution_StartSignalForwarder(t *testing.T) {
	opts := defaultOptions()
	sigChan := make(chan os.Signal, 2)
	state := newExecutionState()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts.startSignalForwarder(ctx, cancel, sigChan, state)

	// Pre-startup signal cancels context
	sigChan <- os.Interrupt

	select {
	case <-ctx.Done():
		// Context successfully cancelled
	case <-time.After(1 * time.Second):
		t.Fatal("expected context to be cancelled on pre-startup signal")
	}
}

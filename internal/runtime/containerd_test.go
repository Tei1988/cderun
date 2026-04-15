package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContainerdRuntime_Name(t *testing.T) {
	// We can't easily run full containerd tests without a socket,
	// but we can test basic methods if we mock the client.
	// For now, let's at least test the Name() method.
	rt := &ContainerdRuntime{}
	assert.Equal(t, "containerd", rt.Name())
}

func TestContainerdRuntime_PullImage_Never(t *testing.T) {
	rt := &ContainerdRuntime{}
	err := rt.PullImage(context.Background(), "alpine", "never", 3, time.Second)
	assert.NoError(t, err)
}

func TestContainerdRuntime_NewContainerdRuntime_ValidSocket(t *testing.T) {
	// client.New() doesn't immediately dial if it's a unix socket,
	// it just sets up the client structure.
	rt, err := NewContainerdRuntime("/tmp/containerd.sock")
	assert.NoError(t, err)
	assert.NotNil(t, rt)
}

func TestParseSignal(t *testing.T) {
	tests := []struct {
		sig      string
		expected int
		wantErr  bool
	}{
		{"SIGKILL", 9, false},
		{"KILL", 9, false},
		{"9", 9, false},
		{"SIGTERM", 15, false},
		{"TERM", 15, false},
		{"15", 15, false},
		{"SIGINT", 2, false},
		{"2", 2, false},
		{"SIGHUP", 1, false},
		{"1", 1, false},
		{"SIGQUIT", 3, false},
		{"3", 3, false},
		{"", 15, false},
		{"INVALID", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			got, err := parseSignal(tt.sig)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, int(got))
			}
		})
	}
}

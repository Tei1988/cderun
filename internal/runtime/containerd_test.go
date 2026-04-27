package runtime

import (
	"context"
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_Name(t *testing.T) {
	r := &ContainerdRuntime{}
	require.Equal(t, "containerd", r.Name())
}

func TestUnit_Containerd_PullImage_Never(t *testing.T) {
	r := &ContainerdRuntime{}
	err := r.PullImage(context.Background(), "test", "never", 3, 1*time.Second)
	require.NoError(t, err)
}

func TestUnit_Containerd_IsRetryablePullError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err      error
		expected bool
	}{
		{fmt.Errorf("toomanyrequests"), true},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("fatal error"), false},
		{nil, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, isRetryablePullError(tt.err))
		})
	}
}

func TestUnit_Containerd_ParseSignal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sig      string
		expected syscall.Signal
		wantErr  bool
	}{
		{"TERM", syscall.SIGTERM, false},
		{"sigterm", syscall.SIGTERM, false},
		{"9", syscall.SIGKILL, false},
		{"KILL", syscall.SIGKILL, false},
		{"INT", syscall.SIGINT, false},
		{"HUP", syscall.SIGHUP, false},
		{"QUIT", syscall.SIGQUIT, false},
		{"invalid", 0, true},
		{"0", 0, true},
		{"65", 0, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.sig, func(t *testing.T) {
			t.Parallel()
			got, err := parseSignal(tt.sig)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, got)
			}
		})
	}
}

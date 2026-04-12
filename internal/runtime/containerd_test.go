package runtime

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ContainerdRuntime_Name(t *testing.T) {
	t.Parallel()
	rt := &ContainerdRuntime{}
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_Containerd_ParseSignal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sig      string
		expected int
		wantErr  bool
	}{
		{"SIGTERM", 15, false},
		{"15", 15, false},
		{"SIGKILL", 9, false},
		{"9", 9, false},
		{"SIGINT", 2, false},
		{"", 15, false},
		{"INVALID", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			t.Parallel()
			got, err := parseSignal(tt.sig)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, int(got))
			}
		})
	}
}

func TestUnit_Runtime_IsRetryablePullError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err      error
		expected bool
	}{
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("some other error"), false},
		{fmt.Errorf("toomanyrequests"), true},
		{fmt.Errorf("token expired"), true},
		{nil, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			assert.Equal(t, tt.expected, isRetryablePullError(tt.err))
		})
	}
}

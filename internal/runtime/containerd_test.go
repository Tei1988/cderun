package runtime

import (
	"fmt"
	"testing"

	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
)

func TestUnit_Containerd_Name(t *testing.T) {
	rt := &ContainerdRuntime{}
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_Containerd_ParseSignal(t *testing.T) {
	tests := []struct {
		sig     string
		wanterr bool
	}{
		{"TERM", false},
		{"sigterm", false},
		{"9", false},
		{"15", false},
		{"HUP", false},
		{"INT", false},
		{"KILL", false},
		{"QUIT", false},
		{"USR1", false},
		{"invalid", true},
		{"0", true},
		{"65", true},
	}

	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			_, err := parseSignal(tt.sig)
			if tt.wanterr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Runtime_IsRetryablePullError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("some other error"), false},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("rate limit exceeded"), true},
		{fmt.Errorf("unauthorized: token expired"), true},
		{errdefs.ErrUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			assert.Equal(t, tt.want, IsRetryablePullError(tt.err))
		})
	}
}

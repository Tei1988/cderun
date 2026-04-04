package runtime

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
)

type mockDockerClientForSignal struct {
	dockerClient
}

func (m *mockDockerClientForSignal) ContainerKill(ctx context.Context, containerID string, signal string) error {
	return nil
}

func TestSignalValidation(t *testing.T) {
	tests := []struct {
		sig     string
		wantErr bool
	}{
		{"SIGTERM", false},
		{"TERM", false},
		{"9", false},
		{"15", false},
		{"SIG15", false},
		{"sigterm", false},
		{"", false},
		{"KILL; rm -rf /", true},
		{"TERM\n", true},
		{"-9", true},
	}

	d := &DockerRuntime{
		client: &mockDockerClientForSignal{},
	}

	for _, tt := range tests {
		err := d.SignalContainer(context.Background(), "test-id", tt.sig)
		if tt.wantErr {
			assert.Error(t, err, "sig: %q", tt.sig)
			assert.Contains(t, err.Error(), "invalid signal")
		} else {
			assert.NoError(t, err, "sig: %q", tt.sig)
		}
	}
}

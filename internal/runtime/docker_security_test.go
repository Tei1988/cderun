package runtime

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDockerClientForSignal struct {
	dockerClient
}

func (m *mockDockerClientForSignal) ContainerKill(ctx context.Context, containerID string, signal string) error {
	return nil
}

func TestSignalValidation(t *testing.T) {
	tests := []struct {
		name    string
		sig     string
		wantErr bool
	}{
		{"Standard SIGTERM", "SIGTERM", false},
		{"Standard TERM", "TERM", false},
		{"Numeric 9", "9", false},
		{"Numeric 15", "15", false},
		{"Standard SIG15", "SIG15", false},
		{"Lowercase sigterm", "sigterm", false},
		{"Empty signal", "", false},
		{"Command injection attempt", "KILL; rm -rf /", true},
		{"Newline injection attempt", "TERM\n", true},
		{"Negative numeric signal", "-9", true},
	}

	d := &DockerRuntime{
		client: &mockDockerClientForSignal{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.SignalContainer(context.Background(), "test-id", tt.sig)
			if tt.wantErr {
				require.Error(t, err, "sig: %q", tt.sig)
				assert.Contains(t, err.Error(), "invalid signal")
			} else {
				require.NoError(t, err, "sig: %q", tt.sig)
			}
		})
	}
}

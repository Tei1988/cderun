package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDaemonInvalidSignal = errors.New("daemon error: invalid signal")

type mockDockerClientForSignal struct {
	dockerClient
	killErr error
}

func (m *mockDockerClientForSignal) Close() error {
	return nil
}

func (m *mockDockerClientForSignal) ContainerKill(ctx context.Context, containerID string, signal string) error {
	return m.killErr
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
		{"Custom signal SIGUSR1", "SIGUSR1", false},
		// "SIGINVALID" matches the regex ^(?i)(SIG[A-Z0-9]+|[A-Z0-9]+|[0-9]+)$,
		// so SignalContainer accepts it and propagates it to the daemon.
		// This confirms that we allow validly formatted but non-existent signal names
		// that pass local regex validation but may be rejected by the daemon.
		{"Validly formatted but non-existent signal", "SIGINVALID", false},
		{"Negative numeric signal", "-9", true},
		{"Injection attempt ; rm -rf", "SIGTERM; rm -rf /", true},
		{"Injection attempt \n", "SIGTERM\n", true},
	}

	mock := &mockDockerClientForSignal{}
	rt := &DockerRuntime{client: mock}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := rt.SignalContainer(context.Background(), "test-id", tt.sig)
			if (err != nil) != tt.wantErr {
				t.Errorf("SignalContainer() error = %v, wantErr %v for sig %q", err, tt.wantErr, tt.sig)
			}
		})
	}
}

func TestSignalContainerDaemonRejection(t *testing.T) {
	t.Parallel()

	// Use mock client with killErr set to a sentinel error to simulate daemon rejection.
	mock := &mockDockerClientForSignal{
		killErr: errDaemonInvalidSignal,
	}
	rt := &DockerRuntime{client: mock}

	err := rt.SignalContainer(context.Background(), "test-id", "SIGINVALID")
	require.Error(t, err)
	assert.ErrorIs(t, err, errDaemonInvalidSignal)
}

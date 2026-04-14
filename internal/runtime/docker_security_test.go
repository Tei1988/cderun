package runtime

import (
	"context"
	"testing"
)

type mockDockerClientForSignal struct {
	dockerClient
}

func (m *mockDockerClientForSignal) Close() error {
	return nil
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
		{"Custom signal SIGUSR1", "SIGUSR1", false},
		{"Invalid signal string", "SIGINVALID", false}, // signalRegex handles it but SignalContainer just passes it if it matches regex. Wait.
		{"Injection attempt ; rm -rf", "SIGTERM; rm -rf /", true},
		{"Injection attempt \n", "SIGTERM\n", true},
	}

	mock := &mockDockerClientForSignal{}
	rt := &DockerRuntime{client: mock}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rt.SignalContainer(context.Background(), "test-id", tt.sig)
			if (err != nil) != tt.wantErr {
				t.Errorf("SignalContainer() error = %v, wantErr %v for sig %q", err, tt.wantErr, tt.sig)
			}
		})
	}
}

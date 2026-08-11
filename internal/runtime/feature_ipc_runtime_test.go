package runtime

import (
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Containerd_ValidateConfig_Ipc(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	tests := []struct {
		name        string
		cfg         *container.ContainerConfig
		errContains string
	}{
		{
			name: "valid empty ipc",
			cfg: &container.ContainerConfig{
				Ipc: "",
			},
		},
		{
			name: "valid private ipc",
			cfg: &container.ContainerConfig{
				Ipc: "private",
			},
		},
		{
			name: "valid host ipc",
			cfg: &container.ContainerConfig{
				Ipc: "host",
			},
		},
		{
			name: "invalid shareable ipc is rejected",
			cfg: &container.ContainerConfig{
				Ipc: "shareable",
			},
			errContains: "unsupported IPC mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rt.ValidateConfig(tt.cfg)
			if tt.errContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

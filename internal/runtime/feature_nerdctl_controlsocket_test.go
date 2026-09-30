package runtime_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

func TestUnit_Nerdctl_ControlSocket_ArgumentInjectionSafety(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nerdctl_safety.sock")

	logger := logging.NewLogger()
	ctrlServer := controlsocket.NewServer(socketPath, logger)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "nerdctl-safe-001"

	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	nerdctlRt, err := runtime.NewNerdctlRuntime(socketPath)
	require.NoError(t, err)

	adapter := runtime.NewControlSocketRuntimeAdapter(nerdctlRt, client, logger)

	// Inject adversarial flag attempts into ContainerConfig fields
	adversarialConfig := &container.ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"--privileged", "sh", "-c", "whoami"},
		Entrypoint:  []string{"/bin/sh", "--entrypoint-override"},
		Env:         []string{"SAFE_ENV=value", "MALICIOUS=--privileged"},
		Workdir:     "/app",
		User:        "1000:1000",
		SecurityOpt: []string{"no-new-privileges:true"},
	}

	// 1. Verify ValidateConfig passes
	require.NoError(t, adapter.ValidateConfig(adversarialConfig))

	// 2. Dispatch CreateContainer over Control Socket
	id, err := adapter.CreateContainer(ctx, adversarialConfig)
	require.NoError(t, err)
	assert.Equal(t, "nerdctl-safe-001", id)

	// 3. Verify that the dispatched ContainerConfig on mockRt preserves structural fields
	created := mockRt.GetCreatedConfig()
	require.NotNil(t, created)
	assert.Equal(t, "alpine:latest", created.Image)
	assert.Equal(t, []string{"--privileged", "sh", "-c", "whoami"}, created.Command)
	assert.Equal(t, []string{"/bin/sh", "--entrypoint-override"}, created.Entrypoint)
	assert.Contains(t, created.Env, "MALICIOUS=--privileged")

	// 4. Verify CLIArgBuilder verification logic for NerdctlRuntime directly
	builder := runtime.NewCLIArgBuilder()
	builder.AddJoinedFlag("--env", "MALICIOUS=--privileged", false)
	builder.AddPositionals("alpine:latest", "--privileged", "sh")
	argv := builder.Build()

	// Structural verification ensures '--' separates flags from positional arguments
	err = builder.VerifyStructure(argv, -1)
	require.NoError(t, err)

	// Ensure '--' is present in argv before positionals
	doubleDashFound := false
	for _, arg := range argv {
		if arg == "--" {
			doubleDashFound = true
			break
		}
	}
	assert.True(t, doubleDashFound, "CLIArgBuilder argv must include '--' positional separator")
}

func TestUnit_Nerdctl_ControlSocket_LifecycleAndErrorPropagation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nerdctl_lifecycle.sock")

	logger := logging.NewLogger()
	ctrlServer := controlsocket.NewServer(socketPath, logger)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "nerdctl-lc-999"
	mockRt.ExitCode = 127

	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	nerdctlRt, err := runtime.NewNerdctlRuntime(socketPath)
	require.NoError(t, err)

	adapter := runtime.NewControlSocketRuntimeAdapter(nerdctlRt, client, logger)

	assert.Equal(t, "nerdctl-controlsocket", adapter.Name())

	cfg := &container.ContainerConfig{
		Image:   "busybox:latest",
		Command: []string{"nonexistent-command"},
	}

	id, err := adapter.CreateContainer(ctx, cfg)
	require.NoError(t, err)

	err = adapter.StartContainer(ctx, id)
	require.NoError(t, err)

	code, err := adapter.WaitContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 127, code)

	err = adapter.RemoveContainer(ctx, id)
	require.NoError(t, err)
}

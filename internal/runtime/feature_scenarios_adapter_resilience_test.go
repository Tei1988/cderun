package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime/controlsocket"
)

// TestScenarios_MockRuntime_LifecycleAndErrorPropagation verifies full container lifecycle
// and error propagation across MockRuntime hooks.
// Reference: docs/testing/strategy.md & docs/guidelines/testing.md
func TestScenarios_MockRuntime_LifecycleAndErrorPropagation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Successful Lifecycle Execution", func(t *testing.T) {
		mockRt := NewMockRuntime()
		mockRt.CreatedContainerID = "mock-container-xyz"
		mockRt.ExitCode = 0

		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		cID, err := mockRt.CreateContainer(ctx, cfg)
		require.NoError(t, err)
		assert.Equal(t, "mock-container-xyz", cID)

		err = mockRt.StartContainer(ctx, cID)
		require.NoError(t, err)

		exitCode, err := mockRt.WaitContainer(ctx, cID)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)

		err = mockRt.RemoveContainer(ctx, cID)
		require.NoError(t, err)

		assert.Equal(t, "mock-container-xyz", mockRt.GetStartedContainerID())
		assert.Equal(t, "mock-container-xyz", mockRt.GetWaitedContainerID())
		assert.Equal(t, "mock-container-xyz", mockRt.GetRemovedContainerID())
	})

	t.Run("Error Propagation During Lifecycle", func(t *testing.T) {
		mockRt := NewMockRuntime()
		mockRt.CreateErr = errors.New("failed to create container")
		mockRt.StartErr = errors.New("failed to start container")
		mockRt.WaitErr = errors.New("container wait error")
		mockRt.RemoveErr = errors.New("failed to remove container")

		cfg := &container.ContainerConfig{Image: "alpine"}

		_, err := mockRt.CreateContainer(ctx, cfg)
		assert.ErrorContains(t, err, "failed to create container")

		err = mockRt.StartContainer(ctx, "cid")
		assert.ErrorContains(t, err, "failed to start container")

		_, err = mockRt.WaitContainer(ctx, "cid")
		assert.ErrorContains(t, err, "container wait error")

		err = mockRt.RemoveContainer(ctx, "cid")
		assert.ErrorContains(t, err, "failed to remove container")
	})

	t.Run("Signal Forwarding and TTY Resize", func(t *testing.T) {
		mockRt := NewMockRuntime()

		err := mockRt.SignalContainer(ctx, "cid-123", "SIGTERM")
		require.NoError(t, err)

		err = mockRt.ResizeContainerTTY(ctx, "cid-123", 30, 100)
		require.NoError(t, err)

		rows, cols := mockRt.GetTTYSize()
		assert.Equal(t, uint(30), rows)
		assert.Equal(t, uint(100), cols)
	})
}

// TestScenarios_DockerAdapter_Helpers verifies conversion helpers for Docker runtime adapter.
// Reference: docs/features/multi-runtime-support.md & docs/features/direct-container-execution.md
func TestScenarios_DockerAdapter_Helpers(t *testing.T) {
	t.Parallel()

	t.Run("ParseDockerRestartPolicy", func(t *testing.T) {
		rp, err := parseDockerRestartPolicy("always")
		require.NoError(t, err)
		assert.Equal(t, "always", string(rp.Name))

		rp, err = parseDockerRestartPolicy("on-failure:5")
		require.NoError(t, err)
		assert.Equal(t, "on-failure", string(rp.Name))
		assert.Equal(t, 5, rp.MaximumRetryCount)

		_, err = parseDockerRestartPolicy("invalid-policy")
		assert.Error(t, err)
	})

	t.Run("BuildDockerUlimits", func(t *testing.T) {
		ulimits := []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
			{Name: "nproc", Soft: 512, Hard: 512},
		}

		dockerUlimits := buildDockerUlimits(ulimits)
		require.Len(t, dockerUlimits, 2)
		assert.Equal(t, "nofile", dockerUlimits[0].Name)
		assert.Equal(t, int64(1024), dockerUlimits[0].Soft)
		assert.Equal(t, int64(2048), dockerUlimits[0].Hard)
	})

	t.Run("ParseGPUs", func(t *testing.T) {
		devRequests, err := parseGPUs("all")
		require.NoError(t, err)
		assert.Len(t, devRequests, 1)

		devRequests, err = parseGPUs("device=0,1")
		require.NoError(t, err)
		assert.Len(t, devRequests, 1)

		_, err = parseGPUs("invalid-gpu-spec!")
		assert.Error(t, err)
	})
}

// TestScenarios_ContainerdAdapter_Validation verifies containerd configuration validation rules.
// Reference: docs/features/multi-runtime-support.md
func TestScenarios_ContainerdAdapter_Validation(t *testing.T) {
	t.Parallel()

	ctrd := &ContainerdRuntime{}

	t.Run("Valid Config Passes", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		}
		assert.NoError(t, ctrd.ValidateConfig(cfg))
	})

	t.Run("Unsupported Features Flagged as Error", func(t *testing.T) {
		cfgWithRestart := &container.ContainerConfig{
			Image:   "alpine:latest",
			Restart: "always",
		}
		err := ctrd.ValidateConfig(cfgWithRestart)
		assert.Error(t, err, "containerd runtime should reject unsupported restart policy")
	})
}

// TestScenarios_ControlSocket_PayloadSerialization verifies RPC payload struct JSON framing
// and client/server RPC interaction over Unix domain socket.
// Reference: docs/features/nested-execution-control-socket.md
func TestScenarios_ControlSocket_PayloadSerialization(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "cderun_test.sock")

	server := controlsocket.NewServer(socketPath, logging.NewLogger())
	mockRt := NewMockRuntime()
	mockRt.CreatedContainerID = "cs-cid-456"

	server.SetDispatcher(mockRt)
	require.NoError(t, server.Start())
	defer func() { _ = server.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	cfg := &container.ContainerConfig{
		Image:   "node:20",
		Command: []string{"node", "app.js"},
	}

	cid, err := client.CreateContainer(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, "cs-cid-456", cid)

	err = client.StartContainer(ctx, cid)
	require.NoError(t, err)

	status, err := client.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, status)

	err = client.RemoveContainer(ctx, cid)
	require.NoError(t, err)

	createArgs := controlsocket.CreateContainerArgs{Config: cfg}
	data, err := json.Marshal(createArgs)
	require.NoError(t, err)

	var unmarshaled controlsocket.CreateContainerArgs
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "node:20", unmarshaled.Config.Image)
}

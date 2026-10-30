package runtime

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_DockerRuntime_ValidateConfig_ShmSize_BoundaryCases(t *testing.T) {
	t.Parallel()

	rt := &DockerRuntime{}

	t.Run("valid shm-size formats", func(t *testing.T) {
		validSizes := []string{"", "512m", "1g", "64k", "1024b"}
		for _, sz := range validSizes {
			err := rt.ValidateConfig(&container.ContainerConfig{ShmSize: sz})
			assert.NoError(t, err, "shm-size %q should be valid", sz)
		}
	})

	t.Run("invalid shm-size format string", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{ShmSize: "invalid_shm_spec"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")
	})

	t.Run("negative shm-size string error", func(t *testing.T) {
		err := rt.ValidateConfig(&container.ContainerConfig{ShmSize: "-100m"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")
	})
}

func TestUnit_DockerRuntime_ToDockerContainerConfig_GPUs_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("gpus = all", func(t *testing.T) {
		reqs, err := parseGPUs("all")
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, "nvidia", reqs[0].Driver)
		assert.Equal(t, -1, reqs[0].Count)
		assert.Equal(t, [][]string{{"gpu"}}, reqs[0].Capabilities)
	})

	t.Run("gpus = device=0,1", func(t *testing.T) {
		reqs, err := parseGPUs("device=0,1")
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, []string{"0", "1"}, reqs[0].DeviceIDs)
		assert.Equal(t, 0, reqs[0].Count)
	})

	t.Run("gpus = empty device selector", func(t *testing.T) {
		_, err := parseGPUs("device=")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty device selector")
	})

	t.Run("gpus = malformed device ID with trailing comma", func(t *testing.T) {
		_, err := parseGPUs("device=0,")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed device ID")
	})

	t.Run("gpus = malformed device ID with empty element", func(t *testing.T) {
		_, err := parseGPUs("device=0, ,1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed device ID")
	})

	t.Run("gpus = count=2", func(t *testing.T) {
		reqs, err := parseGPUs("count=2")
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, 2, reqs[0].Count)
	})

	t.Run("gpus = empty count selector", func(t *testing.T) {
		_, err := parseGPUs("count=")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty count selector")
	})

	t.Run("gpus = invalid negative count", func(t *testing.T) {
		_, err := parseGPUs("count=-2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid count value")
	})

	t.Run("gpus = count=0", func(t *testing.T) {
		_, err := parseGPUs("count=0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid count value")
	})

	t.Run("gpus = count=abc", func(t *testing.T) {
		_, err := parseGPUs("count=abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid count value")
	})

	t.Run("gpus = malformed grammar or unknown selector", func(t *testing.T) {
		_, err := parseGPUs("invalid_selector=1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown gpu selector or malformed grammar")
	})

	t.Run("empty gpus string", func(t *testing.T) {
		reqs, err := parseGPUs("")
		require.NoError(t, err)
		assert.Nil(t, reqs)
	})
}

func TestUnit_DockerRuntime_ToDockerContainerConfig_RestartPolicy_EdgeCases(t *testing.T) {
	t.Parallel()

	validCases := []struct {
		restart      string
		expectedName dockercontainer.RestartPolicyMode
		expectedMax  int
	}{
		{"no", dockercontainer.RestartPolicyDisabled, 0},
		{"always", dockercontainer.RestartPolicyAlways, 0},
		{"unless-stopped", dockercontainer.RestartPolicyUnlessStopped, 0},
		{"on-failure", dockercontainer.RestartPolicyOnFailure, 0},
		{"on-failure:3", dockercontainer.RestartPolicyOnFailure, 3},
		{"on-failure:0", dockercontainer.RestartPolicyOnFailure, 0},
	}

	for _, tc := range validCases {
		policy, err := parseDockerRestartPolicy(tc.restart)
		require.NoError(t, err, "restart policy %q should be valid", tc.restart)
		assert.Equal(t, tc.expectedName, policy.Name)
		assert.Equal(t, tc.expectedMax, policy.MaximumRetryCount)
	}

	invalidCases := []struct {
		restart      string
		errSubstring string
	}{
		{"invalid_policy", "invalid restart policy"},
		{"always:3", "does not support retry suffix"},
		{"unless-stopped:1", "does not support retry suffix"},
		{"on-failure:", "empty retry suffix"},
		{"on-failure:-1", "must be a non-negative integer"},
		{"on-failure:abc", "must be a non-negative integer"},
		{"on-failure:3:4", "supports at most one retry suffix"},
	}

	for _, tc := range invalidCases {
		_, err := parseDockerRestartPolicy(tc.restart)
		require.Error(t, err, "restart policy %q should be invalid", tc.restart)
		assert.Contains(t, err.Error(), tc.errSubstring)
	}
}

func TestUnit_DockerRuntime_ToDockerContainerConfig_MountsAndLimits(t *testing.T) {
	t.Parallel()

	t.Run("optional mount is rejected", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host", Target: "/container", Optional: true},
			},
		}
		_, _, _, err := toDockerContainerConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "optional mount is not supported")
	})

	t.Run("PidsLimit and Init configuration", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:     "alpine:latest",
			PidsLimit: 500,
			Init:      true,
			Restart:   "on-failure:2",
			GPUs:      "all",
		}
		_, hostCfg, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, hostCfg.PidsLimit)
		assert.Equal(t, int64(500), *hostCfg.PidsLimit)
		require.NotNil(t, hostCfg.Init)
		assert.True(t, *hostCfg.Init)
		assert.Equal(t, dockercontainer.RestartPolicyOnFailure, hostCfg.RestartPolicy.Name)
		assert.Equal(t, 2, hostCfg.RestartPolicy.MaximumRetryCount)
		require.Len(t, hostCfg.DeviceRequests, 1)
		assert.Equal(t, "nvidia", hostCfg.DeviceRequests[0].Driver)
		assert.Equal(t, -1, hostCfg.DeviceRequests[0].Count)
		assert.Equal(t, [][]string{{"gpu"}}, hostCfg.DeviceRequests[0].Capabilities)
		assert.Empty(t, hostCfg.DeviceRequests[0].DeviceIDs)
	})
}

func TestUnit_DockerRuntime_PruneContainers_ActiveFilter(t *testing.T) {
	ctx := context.Background()

	mock := &mockDockerClient{
		listResp: []types.Container{
			{ID: "active-running", State: "running"},
			{ID: "active-restarting", State: "restarting"},
			{ID: "active-paused", State: "paused"},
			{ID: "stopped-exited", State: "exited"},
			{ID: "stopped-created", State: "created"},
		},
	}

	runtime := &DockerRuntime{
		logger:    logging.GetGlobalLogger(),
		client:    mock,
		sleepFunc: noopSleepFunc,
	}

	pruned, err := runtime.PruneContainers(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"stopped-exited", "stopped-created"}, pruned)
	assert.True(t, mock.listOpts.All)
	assert.Equal(t, []string{"cderun=true"}, mock.listOpts.Filters.Get("label"))
}

func TestUnit_PodmanRuntime_InitializationAndSocket(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "p")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	socketPath := filepath.Join(tempDir, "podman.sock")

	received := make(chan struct{}, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case received <- struct{}{}:
			default:
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
		}),
	}

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		_ = server.Close()
	}()

	rt, err := NewPodmanRuntime(socketPath, WithAttachCloseWriteGrace(2*time.Second))
	require.NoError(t, err)
	require.NotNil(t, rt)

	assert.Equal(t, "podman", rt.Name())
	assert.Equal(t, 2*time.Second, rt.attachCloseWriteGrace)

	// Exercise rt.client installed on the runtime directly over the Unix socket
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = rt.client.ContainerList(ctx, dockercontainer.ListOptions{})
	require.NoError(t, err)

	select {
	case <-received:
		// Successfully verified that the constructor-installed client communicates over the expected socket
	case <-time.After(1 * time.Second):
		t.Fatal("expected request to reach the temporary Podman Unix socket listener")
	}
}

func TestUnit_DockerRuntime_PruneContainers_ListError(t *testing.T) {
	ctx := context.Background()
	mock := &mockDockerClient{
		listErr: errors.New("list failed"),
	}
	runtime := &DockerRuntime{
		logger:    logging.GetGlobalLogger(),
		client:    mock,
		sleepFunc: noopSleepFunc,
	}

	_, err := runtime.PruneContainers(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list containers for prune")
}

package controlsocket_test

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime/controlsocket"
)

type mockContainerdDispatcher struct {
	mu             sync.Mutex
	createdConfigs []*container.ContainerConfig
	startedIDs     []string
	waitedIDs      []string
	removedIDs     []string
	signaledIDs    []string
	resizedIDs     []string
	attachedIDs    []string
}

func (m *mockContainerdDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdConfigs = append(m.createdConfigs, config)
	return "containerd-id-123", nil
}

func (m *mockContainerdDispatcher) StartContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedIDs = append(m.startedIDs, containerID)
	return nil
}

func (m *mockContainerdDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.waitedIDs = append(m.waitedIDs, containerID)
	return 0, nil
}

func (m *mockContainerdDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removedIDs = append(m.removedIDs, containerID)
	return nil
}

func (m *mockContainerdDispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.mu.Lock()
	m.attachedIDs = append(m.attachedIDs, containerID)
	m.mu.Unlock()
	if ready != nil {
		close(ready)
	}
	return nil
}

func (m *mockContainerdDispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signaledIDs = append(m.signaledIDs, containerID + ":" + sig)
	return nil
}

func (m *mockContainerdDispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resizedIDs = append(m.resizedIDs, containerID)
	return nil
}

func TestUnit_ControlSocket_ContainerdDispatch(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	logger := logging.NewLogger()
	server := controlsocket.NewServer(socketPath, logger)
	dispatcher := &mockContainerdDispatcher{}
	server.SetDispatcher(dispatcher)

	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. CreateContainer
	cfg := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}
	cid, err := client.CreateContainer(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, "containerd-id-123", cid)

	// 2. StartContainer
	err = client.StartContainer(ctx, cid)
	require.NoError(t, err)

	// 3. AttachContainer
	err = client.AttachContainer(ctx, cid, false, nil, io.Discard, io.Discard, nil)
	require.NoError(t, err)

	// 4. SignalContainer
	err = client.SignalContainer(ctx, cid, "SIGTERM")
	require.NoError(t, err)

	// 5. ResizeContainerTTY
	err = client.ResizeContainerTTY(ctx, cid, 24, 80)
	require.NoError(t, err)

	// 6. WaitContainer
	exitCode, err := client.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// 7. RemoveContainer
	err = client.RemoveContainer(ctx, cid)
	require.NoError(t, err)

	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	assert.Len(t, dispatcher.createdConfigs, 1)
	assert.Equal(t, []string{"containerd-id-123"}, dispatcher.startedIDs)
	assert.Equal(t, []string{"containerd-id-123"}, dispatcher.attachedIDs)
	assert.Equal(t, []string{"containerd-id-123:SIGTERM"}, dispatcher.signaledIDs)
	assert.Equal(t, []string{"containerd-id-123"}, dispatcher.resizedIDs)
	assert.Equal(t, []string{"containerd-id-123"}, dispatcher.waitedIDs)
	assert.Equal(t, []string{"containerd-id-123"}, dispatcher.removedIDs)
}

func TestUnit_ControlSocket_InheritedCeilingPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cderun.sock")

	logger := logging.NewLogger()
	server := controlsocket.NewServer(socketPath, logger)
	dispatcher := &mockContainerdDispatcher{}
	server.SetDispatcher(dispatcher)

	parentConfig := &container.ContainerConfig{
		Privileged: false,
		CapAdd:     []string{"SYS_PTRACE"},
		Network:    "bridge",
		Pid:        "",
		IPC:        "",
		Devices: []container.DeviceMapping{
			{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "r"},
		},
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host/read-only-data", Target: "/data", ReadOnly: true},
		},
		SecurityOpt: []string{"no-new-privileges:true"},
	}
	server.SetParentConfig(parentConfig)

	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	t.Run("Valid child within parent ceiling", func(t *testing.T) {
		validChild := &container.ContainerConfig{
			Image:   "alpine:latest",
			CapAdd:  []string{"SYS_PTRACE"},
			Network: "bridge",
			Devices: []container.DeviceMapping{{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "r"}},
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host/read-only-data/sub", Target: "/data/sub", ReadOnly: true},
			},
			SecurityOpt: []string{"no-new-privileges:true"},
		}
		cid, err := client.CreateContainer(ctx, validChild)
		require.NoError(t, err)
		assert.NotEmpty(t, cid)
	})

	t.Run("Privileged mode escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image:      "alpine:latest",
			Privileged: true,
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "privileged mode requested by nested container but not granted to parent")
	})

	t.Run("Capability escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image:  "alpine:latest",
			CapAdd: []string{"SYS_ADMIN"},
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "capability \"SYS_ADMIN\" requested by nested container but not granted to parent")
	})

	t.Run("Host Network namespace escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image:   "alpine:latest",
			Network: "host",
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host network requested by nested container but not granted to parent")
	})

	t.Run("Host PID namespace escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image: "alpine:latest",
			Pid:   "host",
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host PID namespace requested by nested container but not granted to parent")
	})

	t.Run("Host IPC namespace escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image: "alpine:latest",
			IPC:   "host",
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host IPC namespace requested by nested container but not granted to parent")
	})

	t.Run("Device escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image:   "alpine:latest",
			Devices: []container.DeviceMapping{{PathOnHost: "/dev/fuse", PathInContainer: "/dev/fuse"}},
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device \"/dev/fuse\" requested by nested container but not granted to parent")
	})

	t.Run("Device permission escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image:   "alpine:latest",
			Devices: []container.DeviceMapping{{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "rwm"}},
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device \"/dev/null\" requested by nested container but not granted to parent")
	})

	t.Run("Writable mount escalation on read-only parent source path rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image: "alpine:latest",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host/read-only-data", Target: "/custom_target", ReadOnly: false},
			},
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "writable mount requested by nested container for read-only parent source/target path")
	})

	t.Run("Writable mount escalation on ancestor of read-only parent source path rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image: "alpine:latest",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host", Target: "/custom_host", ReadOnly: false},
			},
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "writable mount requested by nested container for read-only parent source/target path")
	})

	t.Run("SecurityOpt unconfined escalation rejected", func(t *testing.T) {
		child := &container.ContainerConfig{
			Image:       "alpine:latest",
			SecurityOpt: []string{"seccomp=unconfined"},
		}
		_, err := client.CreateContainer(ctx, child)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security option \"seccomp=unconfined\" requested by nested container but not granted to parent")
	})
}

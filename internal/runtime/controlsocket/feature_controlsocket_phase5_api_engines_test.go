package controlsocket_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

type mockDispatcher struct {
	mu sync.Mutex

	createFn func(ctx context.Context, config *container.ContainerConfig) (string, error)
	startFn  func(ctx context.Context, containerID string) error
	waitFn   func(ctx context.Context, containerID string) (int, error)
	removeFn func(ctx context.Context, containerID string) error
	attachFn func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	signalFn func(ctx context.Context, containerID string, sig string) error
	resizeFn func(ctx context.Context, containerID string, rows, cols uint) error

	createdConfig *container.ContainerConfig
	startedID     string
	waitedID      string
	removedID     string
	signaledID    string
	signaledSig   string
	resizedID     string
	resizedRows   uint
	resizedCols   uint
}

func (m *mockDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.mu.Lock()
	m.createdConfig = config
	fn := m.createFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, config)
	}
	return "c-phase5-123", nil
}

func (m *mockDispatcher) StartContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	m.startedID = containerID
	fn := m.startFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID)
	}
	return nil
}

func (m *mockDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.mu.Lock()
	m.waitedID = containerID
	fn := m.waitFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID)
	}
	return 0, nil
}

func (m *mockDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	m.removedID = containerID
	fn := m.removeFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID)
	}
	return nil
}

func (m *mockDispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	if ready != nil {
		close(ready)
	}
	m.mu.Lock()
	fn := m.attachFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID, tty, stdin, stdout, stderr, ready)
	}
	if stdout != nil {
		_, _ = stdout.Write([]byte("phase5 output\n"))
	}
	return nil
}

func (m *mockDispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.signaledID = containerID
	m.signaledSig = sig
	fn := m.signalFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID, sig)
	}
	return nil
}

func (m *mockDispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.mu.Lock()
	m.resizedID = containerID
	m.resizedRows = rows
	m.resizedCols = cols
	fn := m.resizeFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, containerID, rows, cols)
	}
	return nil
}

func (m *mockDispatcher) getCreatedConfig() *container.ContainerConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createdConfig
}

func (m *mockDispatcher) getStartedID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startedID
}

func (m *mockDispatcher) getWaitedID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.waitedID
}

func (m *mockDispatcher) getRemovedID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removedID
}

func (m *mockDispatcher) getSignaled() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.signaledID, m.signaledSig
}

func (m *mockDispatcher) getResized() (string, uint, uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resizedID, m.resizedRows, m.resizedCols
}

// containerdValidationDispatcher exercises the production ContainerdRuntime.ValidateConfig path.
type containerdValidationDispatcher struct {
	mockDispatcher
	ctrd runtime.ContainerdRuntime
}

func (c *containerdValidationDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	// Delegates to production ContainerdRuntime.ValidateConfig
	if err := c.ctrd.ValidateConfig(config); err != nil {
		return "", err
	}
	return c.mockDispatcher.CreateContainer(ctx, config)
}

func setupPhase5Server(t *testing.T, dispatcher controlsocket.ContainerRuntimeDispatcher, parentCfg *container.ContainerConfig) (*controlsocket.Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "cderun-phase5.sock")

	logger := logging.NewLogger()
	server := controlsocket.NewServer(sockPath, logger)
	server.SetDispatcher(dispatcher)
	if parentCfg != nil {
		server.SetParentConfig(parentCfg)
	}

	require.NoError(t, server.Start())
	t.Cleanup(func() {
		_ = server.Close()
	})

	return server, sockPath
}

func TestUnit_ControlSocket_Phase5_ApiEngineParity_DockerAndPodman(t *testing.T) {
	mockDisp := &mockDispatcher{}
	parentCfg := &container.ContainerConfig{
		Image:      "alpine:latest",
		Privileged: false,
		CapAdd:     []string{"SYS_PTRACED"},
		Mounts: []container.Mount{
			{Source: "/tmp", Target: "/tmp", ReadOnly: false},
		},
	}

	_, sockPath := setupPhase5Server(t, mockDisp, parentCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, sockPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. CreateContainer within parent bounds
	childCfg := &container.ContainerConfig{
		Image:  "alpine:latest",
		CapAdd: []string{"SYS_PTRACED"},
		Mounts: []container.Mount{
			{Source: "/tmp/child", Target: "/tmp/child", ReadOnly: false},
		},
	}
	containerID, err := client.CreateContainer(ctx, childCfg)
	require.NoError(t, err)
	assert.Equal(t, "c-phase5-123", containerID)
	assert.Equal(t, "alpine:latest", mockDisp.getCreatedConfig().Image)

	// 2. StartContainer
	require.NoError(t, client.StartContainer(ctx, containerID))
	assert.Equal(t, "c-phase5-123", mockDisp.getStartedID())

	// 3. AttachContainer
	var stdoutBuf bytes.Buffer
	ready := make(chan struct{})
	require.NoError(t, client.AttachContainer(ctx, containerID, false, nil, &stdoutBuf, nil, ready))
	<-ready
	assert.Contains(t, stdoutBuf.String(), "phase5 output")

	// 4. SignalContainer & ResizeContainerTTY
	require.NoError(t, client.SignalContainer(ctx, containerID, "SIGTERM"))
	signaledID, signaledSig := mockDisp.getSignaled()
	assert.Equal(t, "c-phase5-123", signaledID)
	assert.Equal(t, "SIGTERM", signaledSig)

	require.NoError(t, client.ResizeContainerTTY(ctx, containerID, 24, 80))
	resizedID, rows, cols := mockDisp.getResized()
	assert.Equal(t, "c-phase5-123", resizedID)
	assert.Equal(t, uint(24), rows)
	assert.Equal(t, uint(80), cols)

	// 5. WaitContainer & RemoveContainer
	exitCode, err := client.WaitContainer(ctx, containerID)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "c-phase5-123", mockDisp.getWaitedID())

	require.NoError(t, client.RemoveContainer(ctx, containerID))
	assert.Equal(t, "c-phase5-123", mockDisp.getRemovedID())
}

func TestUnit_ControlSocket_Phase5_InheritedCeilingPolicyEnforcement(t *testing.T) {
	parentCfg := &container.ContainerConfig{
		Image:      "ubuntu:22.04",
		Privileged: false,
		CapAdd:     []string{"NET_ADMIN"},
		Network:    "bridge",
		Pid:        "host",
		Sysctls:    map[string]string{"net.ipv4.ip_forward": "1"},
		Mounts: []container.Mount{
			{Source: "/data", Target: "/data", ReadOnly: false},
		},
	}

	mockDisp := &mockDispatcher{}
	_, sockPath := setupPhase5Server(t, mockDisp, parentCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, sockPath)
	require.NoError(t, err)
	defer client.Close()

	// Violation 1: Privileged escalation
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		Privileged: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "privileged mode")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Violation 2: Capability escalation
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		CapAdd: []string{"SYS_ADMIN"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not permitted by parent")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Violation 3: Network namespace escalation (e.g. host when parent is bridge)
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		Network: "host",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host network mode")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Violation 4: Sysctl escalation
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		Sysctls: map[string]string{"net.ipv4.ip_forward": "0"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sysctl")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Violation 5: Mount boundary escalation (outside parent mounts)
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		Mounts: []container.Mount{
			{Source: "/etc/shadow", Target: "/shadow", ReadOnly: false},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside parent mount sources")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Valid child container within parent bounds
	validChild := &container.ContainerConfig{
		CapAdd:  []string{"NET_ADMIN"},
		Network: "bridge",
		Pid:     "host",
		Sysctls: map[string]string{"net.ipv4.ip_forward": "1"},
		Mounts: []container.Mount{
			{Source: "/data/sub", Target: "/data/sub", ReadOnly: false},
		},
	}
	cid, err := client.CreateContainer(ctx, validChild)
	require.NoError(t, err)
	assert.NotEmpty(t, cid)
	assert.NotNil(t, mockDisp.getCreatedConfig())
}

func TestUnit_ControlSocket_Phase5_ContainerdApiEngineParity(t *testing.T) {
	if goruntime.GOOS != "linux" {
		t.Skip("containerd runtime is only supported on Linux")
	}

	ctrdDisp := &containerdValidationDispatcher{}
	_, sockPath := setupPhase5Server(t, ctrdDisp, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, sockPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. Verify that unsupported containerd configuration (e.g. OciRuntime or Init specified)
	// triggers the production ContainerdRuntime.ValidateConfig error and halts dispatch.
	invalidCfg := &container.ContainerConfig{
		Image:      "docker.io/library/alpine:latest",
		OciRuntime: "io.containerd.runc.v2",
	}
	_, err = client.CreateContainer(ctx, invalidCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "containerd runtime: oci-runtime is not supported yet")
	assert.Nil(t, ctrdDisp.getCreatedConfig())

	invalidInitCfg := &container.ContainerConfig{
		Image: "docker.io/library/alpine:latest",
		Init:  true,
	}
	_, err = client.CreateContainer(ctx, invalidInitCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "containerd runtime: init is not supported yet")
	assert.Nil(t, ctrdDisp.getCreatedConfig())

	// 2. Verify that supported containerd configuration succeeds through production validation
	// and preserves create, start, wait, and remove parity assertions.
	validCfg := &container.ContainerConfig{
		Image: "docker.io/library/alpine:latest",
	}
	cid, err := client.CreateContainer(ctx, validCfg)
	require.NoError(t, err)
	assert.Equal(t, "c-phase5-123", cid)
	assert.Equal(t, "docker.io/library/alpine:latest", ctrdDisp.getCreatedConfig().Image)

	err = client.StartContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, "c-phase5-123", ctrdDisp.getStartedID())

	code, err := client.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Equal(t, "c-phase5-123", ctrdDisp.getWaitedID())

	err = client.RemoveContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, "c-phase5-123", ctrdDisp.getRemovedID())
}

func TestUnit_ControlSocket_Phase5_MountPathTraversalAndReadOnlyBoundaries(t *testing.T) {
	parentCfg := &container.ContainerConfig{
		ReadOnly: true,
		Mounts: []container.Mount{
			{Source: "/var/log", Target: "/var/log", ReadOnly: true},
		},
	}

	mockDisp := &mockDispatcher{}
	_, sockPath := setupPhase5Server(t, mockDisp, parentCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, sockPath)
	require.NoError(t, err)
	defer client.Close()

	// Path traversal attempt in mount source
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		Mounts: []container.Mount{
			{Source: "/var/log/../shadow", Target: "/shadow", ReadOnly: true},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Read-write request when parent root is read-only
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{
		Mounts: []container.Mount{
			{Source: "/var/log/app.log", Target: "/var/log/app.log", ReadOnly: false},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
	assert.Nil(t, mockDisp.getCreatedConfig())

	// Valid read-only child mount under parent mount
	cid, err := client.CreateContainer(ctx, &container.ContainerConfig{
		Mounts: []container.Mount{
			{Source: "/var/log/app.log", Target: "/var/log/app.log", ReadOnly: true},
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, cid)
	assert.NotNil(t, mockDisp.getCreatedConfig())
}

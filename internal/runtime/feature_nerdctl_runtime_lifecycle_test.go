package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_NerdctlRuntime_ConstructorAndOptions(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()
	rt, err := NewNerdctlRuntime("/var/run/containerd/containerd.sock",
		WithNerdctlLogger(logger),
		WithNerdctlLogger(nil),
		WithNerdctlExecPath("/usr/local/bin/nerdctl"),
		WithNerdctlExecPath(""),
	)
	require.NoError(t, err)
	assert.Equal(t, "nerdctl", rt.Name())
	assert.Equal(t, "/var/run/containerd/containerd.sock", rt.socketPath)
	assert.Equal(t, "/usr/local/bin/nerdctl", rt.execPath)
	assert.NoError(t, rt.Close())
}

func TestUnit_NerdctlRuntime_ValidateConfig_MountCommaRejection(t *testing.T) {
	t.Parallel()

	rt, err := NewNerdctlRuntime("")
	require.NoError(t, err)

	// Valid config
	validCfg := &container.ContainerConfig{
		Image: "alpine:latest",
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host", Target: "/container"},
		},
	}
	assert.NoError(t, rt.ValidateConfig(validCfg))

	// Invalid type with comma
	badTypeCfg := &container.ContainerConfig{
		Image: "alpine:latest",
		Mounts: []container.Mount{
			{Type: "bind,ro", Source: "/host", Target: "/container"},
		},
	}
	err = rt.ValidateConfig(badTypeCfg)
	assert.ErrorContains(t, err, "mount type \"bind,ro\" contains invalid comma")

	// Invalid source with comma
	badSrcCfg := &container.ContainerConfig{
		Image: "alpine:latest",
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host,foo", Target: "/container"},
		},
	}
	err = rt.ValidateConfig(badSrcCfg)
	assert.ErrorContains(t, err, "mount source \"/host,foo\" contains invalid comma")

	// Invalid target with comma
	badDstCfg := &container.ContainerConfig{
		Image: "alpine:latest",
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host", Target: "/container,bar"},
		},
	}
	err = rt.ValidateConfig(badDstCfg)
	assert.ErrorContains(t, err, "mount target \"/container,bar\" contains invalid comma")
}

func TestUnit_NerdctlRuntime_PullImage_Scenarios(t *testing.T) {
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl.sh")

	// Mock script that returns success or error based on args
	scriptContent := `#!/bin/sh
if [ "$1" = "--address=/sock" ]; then
    shift
fi
case "$1" in
    image)
        if [ "$3" = "existing:latest" ]; then
            exit 0
        fi
        exit 1
        ;;
    pull)
        if [ "$2" = "--" ]; then
            img="$3"
        else
            img="$2"
        fi
        if [ "$img" = "fail-pull:latest" ]; then
            echo "network connection error" >&2
            exit 1
        fi
        if [ "$img" = "fatal-pull:latest" ]; then
            echo "image not found" >&2
            exit 1
        fi
        echo "Pulled $img"
        exit 0
        ;;
    *)
        exit 1
        ;;
esac
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	ctx := context.Background()

	// 1. Policy "never"
	assert.NoError(t, rt.PullImage(ctx, "any:latest", "never", 1, 10*time.Millisecond))

	// 2. Policy "missing" - image exists locally
	assert.NoError(t, rt.PullImage(ctx, "existing:latest", "missing", 1, 10*time.Millisecond))

	// 3. Policy "missing" - image missing locally, pull succeeds
	assert.NoError(t, rt.PullImage(ctx, "missing-ok:latest", "missing", 1, 10*time.Millisecond))

	// 4. Policy "always" - pull succeeds
	assert.NoError(t, rt.PullImage(ctx, "always-ok:latest", "always", 1, 10*time.Millisecond))

	// 5. Retryable failure max retries exceeded
	err = rt.PullImage(ctx, "fail-pull:latest", "always", 2, 5*time.Millisecond)
	assert.ErrorContains(t, err, "failed to pull image fail-pull:latest with nerdctl")

	// 6. Canceled context before pull
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	assert.Equal(t, context.Canceled, rt.PullImage(canceledCtx, "always-ok:latest", "always", 1, 10*time.Millisecond))
}

func TestUnit_NerdctlRuntime_CreateContainer_FormattingAndErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl-create.sh")

	scriptContent := `#!/bin/sh
if [ "$1" = "--address=/sock" ]; then
    shift
fi
if [ "$1" = "create" ]; then
    shift
    # Check if special test trigger is present
    for arg in "$@"; do
        if [ "$arg" = "fail-create:latest" ]; then
            echo "creation failed due to missing resource" >&2
            exit 125
        fi
    done
    echo "c1234567890"
    exit 0
fi
exit 1
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	ctx := context.Background()

	// Full config creation test
	fullCfg := &container.ContainerConfig{
		Image:      "ubuntu:22.04",
		Entrypoint: []string{"/bin/bash", "-c"},
		Command:    []string{"echo hello"},
		Workdir:    "/app",
		User:       "1000:1000",
		Hostname:   "test-host",
		Network:    "bridge",
		IPC:        "host",
		Cgroupns:   "host",
		Restart:    "always",
		ShmSize:    "64m",
		CpusetCpus: "0-1",
		CpusetMems: "0",
		GPUs:       "all",
		Pid:        "host",
		OciRuntime: "runc",
		CPUs:       2.5,
		PidsLimit:  100,
		CPUShares:  512,
		Memory:     1024 * 1024 * 512,
		TTY:        true,
		Interactive: true,
		ReadOnly:   true,
		Init:       true,
		Privileged: true,
		PublishAll: true,
		Env:        []string{"FOO=bar"},
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host", Target: "/container", ReadOnly: true},
		},
		CapAdd:      []string{"SYS_ADMIN"},
		CapDrop:     []string{"NET_RAW"},
		SecurityOpt: []string{"no-new-privileges:true"},
		DNS:         []string{"8.8.8.8"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2"},
		AddHosts:    []string{"customhost:127.0.0.1"},
		Ports:       []string{"8080:80"},
		Expose:      []string{"8080/tcp"},
		GroupAdd:    []string{"1001"},
		Devices: []container.DeviceMapping{
			{PathOnHost: "/dev/kvm", PathInContainer: "/dev/kvm", CgroupPermissions: "rwm"},
			{PathOnHost: "/dev/snd", PathInContainer: "/dev/sound", CgroupPermissions: "rw"},
			{PathOnHost: "/dev/null", CgroupPermissions: "r"},
		},
		Ulimits: []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
		Sysctls: map[string]string{"net.ipv4.ip_forward": "1"},
		Labels:  map[string]string{"env": "test"},
	}

	cid, err := rt.CreateContainer(ctx, fullCfg)
	require.NoError(t, err)
	assert.Equal(t, "c1234567890", cid)

	// Failure test unwrapping stderr
	failCfg := &container.ContainerConfig{
		Image: "fail-create:latest",
	}
	_, err = rt.CreateContainer(ctx, failCfg)
	assert.ErrorContains(t, err, "nerdctl create failed")
	assert.ErrorContains(t, err, "creation failed due to missing resource")
}

func TestUnit_NerdctlRuntime_LifecycleAndOperations(t *testing.T) {
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl-ops.sh")

	scriptContent := `#!/bin/sh
if [ "$1" = "--address=/sock" ]; then
    shift
fi
cmd="$1"
shift

target=""
for arg in "$@"; do
    target="$arg"
done

case "$cmd" in
    start)
        if [ "$target" = "bad-id" ]; then
            echo "no such container" >&2
            exit 1
        fi
        echo "started $target"
        exit 0
        ;;
    wait)
        if [ "$target" = "bad-id" ]; then
            echo "no such container" >&2
            exit 1
        elif [ "$target" = "invalid-code" ]; then
            echo "not-a-number"
            exit 0
        fi
        echo "0"
        exit 0
        ;;
    rm)
        if [ "$target" = "bad-id" ]; then
            echo "no such container" >&2
            exit 1
        fi
        echo "removed"
        exit 0
        ;;
    kill)
        if [ "$target" = "bad-id" ]; then
            echo "no such container" >&2
            exit 1
        fi
        echo "signaled"
        exit 0
        ;;
    inspect)
        if [ "$target" = "running-id" ]; then
            echo "true 0"
            exit 0
        elif [ "$target" = "stopped-id" ]; then
            echo "false 137"
            exit 0
        elif [ "$target" = "invalid-inspect" ]; then
            echo "invalid_output"
            exit 0
        elif [ "$target" = "invalid-exitcode" ]; then
            echo "false abc"
            exit 0
        fi
        echo "container not found" >&2
        exit 1
        ;;
    container)
        if [ "$1" = "prune" ]; then
            echo "Deleted Containers:"
            echo "c1"
            echo "c2"
            exit 0
        fi
        exit 1
        ;;
    attach)
        if [ "$target" = "bad-id" ]; then
            exit 1
        fi
        exit 0
        ;;
    *)
        exit 1
        ;;
esac
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("/sock", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	ctx := context.Background()

	// 1. StartContainer
	assert.NoError(t, rt.StartContainer(ctx, "c123"))
	err = rt.StartContainer(ctx, "bad-id")
	assert.ErrorContains(t, err, "nerdctl start failed")
	assert.ErrorContains(t, err, "no such container")

	// 2. WaitContainer
	code, err := rt.WaitContainer(ctx, "c123")
	require.NoError(t, err)
	assert.Equal(t, 0, code)

	_, err = rt.WaitContainer(ctx, "bad-id")
	assert.ErrorContains(t, err, "nerdctl wait failed")

	_, err = rt.WaitContainer(ctx, "invalid-code")
	assert.ErrorContains(t, err, "failed to parse nerdctl wait exit code")

	// 3. RemoveContainer
	assert.NoError(t, rt.RemoveContainer(ctx, "c123"))
	assert.ErrorContains(t, rt.RemoveContainer(ctx, "bad-id"), "nerdctl rm failed")

	// 4. SignalContainer
	assert.NoError(t, rt.SignalContainer(ctx, "c123", "SIGTERM"))
	assert.ErrorContains(t, rt.SignalContainer(ctx, "bad-id", "SIGKILL"), "nerdctl kill failed")

	// 5. InspectContainer
	running, exitCode, err := rt.InspectContainer(ctx, "running-id")
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, 0, exitCode)

	running, exitCode, err = rt.InspectContainer(ctx, "stopped-id")
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 137, exitCode)

	_, _, err = rt.InspectContainer(ctx, "bad-id")
	assert.ErrorContains(t, err, "nerdctl inspect failed")

	_, _, err = rt.InspectContainer(ctx, "invalid-inspect")
	assert.ErrorContains(t, err, "malformed nerdctl inspect output")

	_, _, err = rt.InspectContainer(ctx, "invalid-exitcode")
	assert.ErrorContains(t, err, "failed to parse exit code from inspect output")

	// 6. PruneContainers
	pruned, err := rt.PruneContainers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"c1", "c2"}, pruned)

	// 7. AttachContainer
	ready := make(chan struct{})
	var stdout bytes.Buffer
	assert.NoError(t, rt.AttachContainer(ctx, "c123", false, nil, &stdout, nil, ready))
	_, isClosed := <-ready
	assert.False(t, isClosed)

	// 8. ResizeContainerTTY
	assert.NoError(t, rt.ResizeContainerTTY(ctx, "c123", 24, 80))
}

func TestUnit_NerdctlRuntime_PruneErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "mock-nerdctl-prune-fail.sh")

	scriptContent := `#!/bin/sh
exit 1
`
	require.NoError(t, os.WriteFile(fakeExec, []byte(scriptContent), 0755))

	rt, err := NewNerdctlRuntime("", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)

	_, err = rt.PruneContainers(context.Background())
	assert.ErrorContains(t, err, "nerdctl container prune failed")
}

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_NerdctlRuntime_AdversarialArgumentInjection(t *testing.T) {
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "fake-nerdctl.sh")

	// Script that writes raw received arguments to stdout
	scriptContent := "#!/bin/sh\nfor arg in \"$@\"; do echo \"ARG:$arg\"; done\n"
	err := os.WriteFile(fakeExec, []byte(scriptContent), 0755)
	require.NoError(t, err)

	rt, err := NewNerdctlRuntime("", WithNerdctlExecPath(fakeExec))
	require.NoError(t, err)
	assert.Equal(t, "nerdctl", rt.Name())

	t.Run("CreateContainer neutralizes malicious flag strings in fields", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:      "--privileged", // Attempt argument injection via Image
			Command:    []string{"--mount=type=bind,src=/,dst=/host", "sh"},
			Env:        []string{"EVIL=--entrypoint=/bin/sh"},
			Labels:     map[string]string{"INJECT": "--cap-add=SYS_ADMIN"},
			Workdir:    "--workdir=/root",
			Network:    "--net=host",
			User:       "--user=0:0",
			Hostname:   "--hostname=pwned",
			ShmSize:    "--shm-size=10g",
			CpusetCpus: "--cpuset-cpus=0-3",
			GPUs:       "--gpus=all",
			CapAdd:     []string{"--cap-add=ALL"},
		}

		cID, err := rt.CreateContainer(context.Background(), cfg)
		require.NoError(t, err)

		// The output contains lines of ARG:<arg>
		lines := strings.Split(strings.TrimSpace(cID), "\n")
		rawArgs := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.HasPrefix(line, "ARG:") {
				rawArgs = append(rawArgs, strings.TrimPrefix(line, "ARG:"))
			}
		}

		// Verify that "--" appears before "--privileged" positional argument
		doubleDashIdx := -1
		for i, arg := range rawArgs {
			if arg == "--" {
				doubleDashIdx = i
				break
			}
		}
		require.NotEqual(t, -1, doubleDashIdx, "expected '--' positional separator in raw args: %v", rawArgs)
		require.Greater(t, len(rawArgs), doubleDashIdx+1)
		assert.Equal(t, "--privileged", rawArgs[doubleDashIdx+1])

		// Re-test via CLIArgBuilder verification
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--workdir", cfg.Workdir, true)
		builder.AddJoinedFlag("--user", cfg.User, true)
		builder.AddJoinedFlag("--hostname", cfg.Hostname, true)
		builder.AddJoinedFlag("--net", cfg.Network, true)
		builder.AddJoinedFlag("--shm-size", cfg.ShmSize, true)
		builder.AddJoinedFlag("--cpuset-cpus", cfg.CpusetCpus, true)
		builder.AddJoinedFlag("--gpus", cfg.GPUs, true)
		for _, e := range cfg.Env {
			builder.AddJoinedFlag("--env", e, false)
		}
		for _, cap := range cfg.CapAdd {
			builder.AddJoinedFlag("--cap-add", cap, false)
		}
		for k, v := range cfg.Labels {
			builder.AddJoinedFlag("--label", k+"="+v, false)
		}
		positionals := []string{cfg.Image}
		positionals = append(positionals, cfg.Command...)
		builder.AddPositionals(positionals...)

		built := builder.Build()
		err = builder.VerifyStructure(built, -1)
		require.NoError(t, err)

		// Confirm "--privileged" is placed after "--" as a positional argument, not as a root flag
		builtDoubleDashIdx := -1
		for i, arg := range built {
			if arg == "--" {
				builtDoubleDashIdx = i
				break
			}
		}
		require.NotEqual(t, -1, builtDoubleDashIdx)
		assert.Equal(t, "--privileged", built[builtDoubleDashIdx+1])
	})

	t.Run("ValidateConfig returns error for empty image or nil config", func(t *testing.T) {
		assert.Error(t, rt.ValidateConfig(nil))
		assert.Error(t, rt.ValidateConfig(&container.ContainerConfig{Image: ""}))
		assert.NoError(t, rt.ValidateConfig(&container.ContainerConfig{Image: "alpine"}))
	})

	t.Run("PullImage rejects invalid pull policy", func(t *testing.T) {
		err := rt.PullImage(context.Background(), "alpine", "invalid-policy", 1, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown pull policy")
	})

	t.Run("ValidateConfig rejects mount source traversal", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/tmp/../etc/passwd", Target: "/etc/passwd"},
			},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal")
	})

	t.Run("ValidateConfig rejects mount target traversal", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/tmp/data", Target: "/var/../etc/passwd"},
			},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal")
	})

	t.Run("ValidateConfig rejects host device traversal", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/../etc/shadow", PathInContainer: "/dev/shadow"},
			},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal")
	})

	t.Run("ValidateConfig rejects container device traversal", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/null", PathInContainer: "/dev/../etc/shadow"},
			},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory traversal")
	})

	t.Run("ValidateConfig accepts valid paths with double dot substrings", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/tmp/a..b", Target: "/var/a..b"},
			},
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/a..b", PathInContainer: "/dev/c..d"},
			},
		}
		err := rt.ValidateConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("ValidateConfig rejects environment null bytes", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			Env:   []string{"FOO=bar\x00baz"},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contains null byte")
	})
}

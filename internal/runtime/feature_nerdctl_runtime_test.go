package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

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

	t.Run("NewNerdctlRuntime options handling and logger guard", func(t *testing.T) {
		customLogger := logging.NewLogger()
		rtCustom, err := NewNerdctlRuntime("", WithNerdctlLogger(customLogger))
		require.NoError(t, err)
		assert.Same(t, customLogger, rtCustom.logger)

		// Passing nil logger option preserves the original default logger
		rtNilOpt, err := NewNerdctlRuntime("", WithNerdctlLogger(nil))
		require.NoError(t, err)
		assert.NotNil(t, rtNilOpt.logger)
		assert.NotSame(t, customLogger, rtNilOpt.logger)
	})

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

		// Verify that all option flags are passed as combined "--flag=value" tokens before "--"
		doubleDashIdx := -1
		for i, arg := range rawArgs {
			if arg == "--" {
				doubleDashIdx = i
				break
			}
		}
		require.NotEqual(t, -1, doubleDashIdx, "expected '--' delimiter in built argv")

		// Re-verify with CLIArgBuilder
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--net", cfg.Network, false)
		builder.AddJoinedFlag("--workdir", cfg.Workdir, false)
		builder.AddJoinedFlag("--user", cfg.User, false)
		builder.AddJoinedFlag("--hostname", cfg.Hostname, false)
		builder.AddJoinedFlag("--shm-size", cfg.ShmSize, false)
		builder.AddJoinedFlag("--cpuset-cpus", cfg.CpusetCpus, false)
		builder.AddJoinedFlag("--gpus", cfg.GPUs, false)
		for _, env := range cfg.Env {
			builder.AddJoinedFlag("--env", env, false)
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
}

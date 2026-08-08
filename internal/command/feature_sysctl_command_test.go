package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Sysctl_DryRun(t *testing.T) {
	t.Parallel()

	t.Run("dry-run with --sysctl outputs sysctls in yaml", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--sysctl", "net.ipv4.ip_forward=1", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "sysctls:")
		assert.Contains(t, output, "net.ipv4.ip_forward: \"1\"")
	})

	t.Run("dry-run with --cderun-sysctl overrides --sysctl in yaml", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--sysctl", "net.ipv4.ip_forward=0", "--image", "alpine", "sh", "--cderun-sysctl=net.ipv4.ip_forward=1", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "sysctls:")
		assert.Contains(t, output, "net.ipv4.ip_forward: \"1\"")
	})

	t.Run("dry-run simple format outputs Sysctls: ...", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--dry-run-format", "simple", "--sysctl", "net.ipv4.ip_forward=1", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Sysctls: net.ipv4.ip_forward=1")
	})
}

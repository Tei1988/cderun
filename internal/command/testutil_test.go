package command

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

func skipIfDockerBroken(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "failed to mount") && strings.Contains(msg, "invalid argument") {
		t.Skip("Skipping test due to Docker mount limitation in this environment (likely overlay-on-overlay)")
	}
	if strings.Contains(msg, "Data limit exceeded") || strings.Contains(msg, "pull rate limit") {
		t.Skip("Skipping test due to Docker Hub rate limit")
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "connection refused") {
		t.Skipf("Skipping test due to transient network/runtime issue: %v", err)
	}
	// Detect Docker SIGKILL timeout (likely environment resource constraint or slow CI)
	if strings.Contains(msg, "timeout") && strings.Contains(msg, "SIGKILL") {
		t.Skip("Skipping test due to Docker SIGKILL timeout (likely environment resource constraint)")
	}
}

var setupMu sync.Mutex

func withMockRuntime(mock runtime.ContainerRuntime, extras ...func(o *rootOptions, cmd *cobra.Command)) func(o *rootOptions, cmd *cobra.Command) {
	return func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		// Default exitFunc to a no-op to prevent tests from terminating the process.
		// Callers can still override this behavior by providing an extra setup function.
		o.exitFunc = func(code int) {}

		for _, extra := range extras {
			extra(o, cmd)
		}
	}
}

func setupTestDir(t *testing.T) string {
	t.Helper()

	// Use a unique subdirectory within the test's temp directory
	// to ensure absolute isolation for filesystem-dependent tests.
	tmpDir := t.TempDir()

	// Note: We still use Chdir here for legacy integration tests that don't yet
	// fully use the MockFileSystem. For these tests, we must NOT call t.Parallel().
	// We serialize access to the global working directory using setupMu to reduce flakes.
	// We hold the lock for the entire lifetime of the test that changes CWD.
	setupMu.Lock()
	restoreWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))

	t.Cleanup(func() {
		defer setupMu.Unlock()
		_ = os.Chdir(restoreWd)
	})
	return tmpDir
}

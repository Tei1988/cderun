package command

import (
	"os"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

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

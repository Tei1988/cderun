//go:build runtime

package command

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var setupMu sync.Mutex

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
	var restoreWd string
	t.Cleanup(func() {
		defer setupMu.Unlock()
		if restoreWd != "" {
			require.NoError(t, os.Chdir(restoreWd))
		}
	})

	var err error
	restoreWd, err = os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	return tmpDir
}

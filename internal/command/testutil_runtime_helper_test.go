//go:build runtime

package command

import (
	"os"
	"strings"
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

// checkRuntimeResult is a helper to standardize result checking in E2E tests.
// It uses isRuntimeBackendError and skipIfRuntimeBroken to handle environmental limitations reported via error or stderr.
func checkRuntimeResult(t *testing.T, stdout, stderr string, exitCode int, err error) {
	t.Helper()
	if isRuntimeBackendError(strings.ToLower(stderr)) {
		t.Skipf("Skipping test due to runtime/backend limitation or setup issue in stderr: %s", stderr)
	}
	skipIfRuntimeBroken(t, err)
	if err != nil {
		t.Fatalf("command returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("command failed with exit code %d: %s", exitCode, stderr)
	}
}

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

	tmpDir := t.TempDir()

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

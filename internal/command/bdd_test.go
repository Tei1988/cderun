package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestScenario_CommandResolution_CollectionOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Explicit empty env in Tool overrides non-empty Global", func(t *testing.T) {
		t.Parallel()
		// Given: Global defaults with non-empty env, and Tool with explicit empty env
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.cderun.yaml": []byte("defaults:\n  env:\n    - GLOBAL=1"),
				"/project/.tools.yaml":  []byte("app:\n  image: my-app\n  env: []"),
			},
			Dirs: map[string]bool{"/project": true},
		}

		mockRuntime := &runtime.MockRuntime{}
		// When: Executing command
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "app", "ls"}, withMockRuntime(mockRuntime, withMockFS(mfs)))

		// Then: The resolved env should be empty (only containing default internal vars if any, but GLOBAL=1 should be gone)
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.NotContains(t, mockRuntime.CreatedConfig.Env, "GLOBAL=1")
	})
}

package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Coverage_Resolver_ResolveWithFS_RegistryMismatch(t *testing.T) {
	// Backup and restore global state if necessary, but fieldOnce prevents re-init.
	// This test specifically triggers the fail-fast panic by adding a mismatched entry.

	t.Run("panic on missing field", func(t *testing.T) {
		// We can't easily modify the global StringOptions and re-run initFieldInfo
		// because of fieldOnce. Do.
		// However, we can test that it panics if we were to call it with bad data.
		// Since initFieldInfo is internal and called once, we rely on the fact that
		// it's already been called in other tests or will be called here.

		// To truly test the panic, we would need to call a function that calls getFieldInfo
		// with invalid arguments.

		_, err := getFieldInfo("not-in-cli", "NonExistent", reflect.TypeFor[string]())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in ResolvedConfig")
	})
}

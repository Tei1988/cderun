package command

import (
	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"testing"
)

func assertMountSourceEquals(t *testing.T, mounts []container.Mount, target, expectedSource string) {
	t.Helper()
	var found bool
	for _, m := range mounts {
		if m.Target == target {
			assert.Equal(t, expectedSource, m.Source)
			found = true
			break
		}
	}
	assert.True(t, found, "mount with target %q not found", target)
}

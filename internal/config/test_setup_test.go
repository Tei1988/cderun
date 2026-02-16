package config

import (
	"testing"
)

func setupNoOverlay(t *testing.T) {
	t.Helper()
	originalReader := DefaultMountInfoReader
	DefaultMountInfoReader = &mockMountInfoReader{Content: []byte("")}
	t.Cleanup(func() { DefaultMountInfoReader = originalReader })
}

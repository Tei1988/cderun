package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathResolution(t *testing.T) {
	home, _ := os.UserHomeDir()
	baseDir := "/abs/path"
	r, err := NewExpressionResolver()
	require.NoError(t, err)

	t.Run("resolvePath", func(t *testing.T) {
		assert.Equal(t, "/abs/path/file", resolvePath("./file", baseDir))
		assert.Equal(t, "/abs/file", resolvePath("../file", baseDir))
		assert.Equal(t, filepath.Join(home, ".ssh"), resolvePath("~/.ssh", baseDir))
		assert.Equal(t, "/other/abs/path", resolvePath("/other/abs/path", baseDir))
		assert.Equal(t, "just-name", resolvePath("just-name", baseDir)) // No ./ prefix, no resolution
	})

	t.Run("ConfigPath.Resolve", func(t *testing.T) {
		cp := ConfigPath{Raw: "./data", BaseDir: baseDir}
		assert.Equal(t, "/abs/path/data", cp.Resolve(r))

		cp = ConfigPath{Raw: "{{HOME}}/config", BaseDir: baseDir}
		assert.Equal(t, filepath.Join(home, "config"), cp.Resolve(r))
	})

	t.Run("ConfigPath.ResolveVolume", func(t *testing.T) {
		cp := ConfigPath{Raw: "./data:/app/data", BaseDir: baseDir}
		assert.Equal(t, "/abs/path/data:/app/data", cp.ResolveVolume(r))

		cp = ConfigPath{Raw: "~/config:/root/config:ro", BaseDir: baseDir}
		assert.Equal(t, filepath.Join(home, "config")+":/root/config:ro", cp.ResolveVolume(r))
	})

	t.Run("Windows Paths", func(t *testing.T) {
		cp := ConfigPath{Raw: `C:\host\path:/container`, BaseDir: baseDir}
		assert.Equal(t, `C:\host\path:/container`, cp.ResolveVolume(r))

		cp = ConfigPath{Raw: `E:\dev\path:/dev/path`, BaseDir: baseDir}
		assert.Equal(t, `E:\dev\path:/dev/path`, cp.ResolveDevice(r))
	})
}

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathResolution(t *testing.T) {
	home, _ := os.UserHomeDir()
	baseDir := "/abs/path"

	t.Run("resolvePath", func(t *testing.T) {
		assert.Equal(t, "/abs/path/file", resolvePath("./file", baseDir))
		assert.Equal(t, "/abs/file", resolvePath("../file", baseDir))
		assert.Equal(t, filepath.Join(home, ".ssh"), resolvePath("~/.ssh", baseDir))
		assert.Equal(t, "/other/abs/path", resolvePath("/other/abs/path", baseDir))
		assert.Equal(t, "just-name", resolvePath("just-name", baseDir)) // No ./ prefix, no resolution
	})

	t.Run("ResolvePathsTool", func(t *testing.T) {
		cfg := &ToolConfig{
			Volumes: []string{
				"./data:/app/data",
				"~/config:/root/config:ro",
				"/abs:/abs",
			},
		}
		ResolvePathsTool(cfg, baseDir)

		assert.Equal(t, "/abs/path/data:/app/data", cfg.Volumes[0])
		assert.Equal(t, filepath.Join(home, "config") + ":/root/config:ro", cfg.Volumes[1])
		assert.Equal(t, "/abs:/abs", cfg.Volumes[2])
	})
}

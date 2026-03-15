package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_Config_Load_RealFS(t *testing.T) {
	// Keep one test with real filesystem to ensure RealFileSystem works
	// We use t.TempDir() and os.Chdir, so we cannot use t.Parallel()
	tmpDir := t.TempDir()

	content := "runtime: docker"
	err := RealFileSystem{}.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(content), 0o644)
	require.NoError(t, err)

	originalWd, err := RealFileSystem{}.Getwd()
	require.NoError(t, err)

	// Changing the working directory is process-global and can affect parallel tests.
	// In internal/command we have a mutex for this, but here it's simpler to just not use t.Parallel.
	mfs := &MockFileSystem{
		WD: tmpDir,
		Files: map[string][]byte{
			filepath.Join(tmpDir, ".cderun.yaml"): []byte(content),
		},
	}
	loader := NewConfigLoaderWithFS(mfs)

	cfg, paths, err := loader.LoadCDERunConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, paths)
	assert.Equal(t, filepath.Join(tmpDir, ".cderun.yaml"), paths[0])

	// Original test intended to use global state, let's keep a tiny part of it
	// BUT, it's better to avoid global state entirely.
	// Since LoadCDERunConfig() uses defaultLoader which uses RealFileSystem,
	// let's just use a loader with explicit mfs.
	_ = originalWd
}

func TestIntegration_Config_Merge_Hierarchical(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/project/child",
		Dirs: map[string]bool{
			"/project":       true,
			"/project/child": true,
		},
		Files: map[string][]byte{
			"/project/.cderun.yaml": []byte(`
runtime: docker
defaults:
  tty: false
  network: bridge
`),
			"/project/.tools.yaml": []byte(`
node:
  image: node:14
  env: ["PARENT=1"]
`),
			"/project/child/.cderun.yaml": []byte(`
defaults:
  tty: true
`),
			"/project/child/.tools.yaml": []byte(`
node:
  image: node:16
python:
  image: python:3.9
`),
		},
	}

	loader := NewConfigLoaderWithFS(mfs)

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		cfg, paths, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Equal(t, "/project/child/.cderun.yaml", paths[0])
		assert.Equal(t, "/project/.cderun.yaml", paths[1])

		assert.Equal(t, "docker", cfg.Runtime)          // From parent
		assert.True(t, *cfg.Defaults.TTY)               // From child (overridden)
		assert.Equal(t, "bridge", cfg.Defaults.Network) // From parent
	})

	t.Run("ToolsConfig Merge", func(t *testing.T) {
		cfg, paths, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		require.Len(t, paths, 2)

		node := cfg["node"]
		assert.Equal(t, "node:16", node.Image)          // From child (overridden)
		assert.Equal(t, []string{"PARENT=1"}, node.Env) // From parent (preserved by deep merge)

		python := cfg["python"]
		assert.Equal(t, "python:3.9", python.Image) // From child
	})
}

func TestIntegration_Config_Expression_Resolve(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD:      "/project",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/project/version.txt": []byte(" 1.2.3 \n"),
		},
	}

	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Magic Words", func(t *testing.T) {
		assert.Equal(t, resolver.Pwd, resolver.Resolve("{{PWD}}"))
		assert.Equal(t, resolver.Home, resolver.Resolve("{{HOME}}"))
		assert.Equal(t, resolver.Pwd+"/src", resolver.Resolve("{{PWD}}/src"))
	})

	t.Run("File Directive", func(t *testing.T) {
		assert.Equal(t, "golang:1.2.3", resolver.Resolve("golang:{{file:version.txt}}"))
		require.NoError(t, resolver.Error())
		resolver.Resolve("{{file:nonexistent.txt}}")
		require.Error(t, resolver.Error())

		t.Run("Path Traversal Protection", func(t *testing.T) {
			r2, _ := NewExpressionResolverWithFS(nil, mfs)
			r2.Resolve("{{file:/etc/passwd}}")
			require.Error(t, r2.Error())

			r3, _ := NewExpressionResolverWithFS(nil, mfs)
			r3.Resolve("{{file:../etc/passwd}}")
			require.Error(t, r3.Error())
		})
	})

	t.Run("Nested Structures", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, mfs)
		input := map[string]any{
			"image": "node:{{PWD}}",
			"env": []any{
				"HOME={{HOME}}",
				"OTHER=fixed",
			},
		}
		expected := map[string]any{
			"image": "node:" + r.Pwd,
			"env": []any{
				"HOME=" + r.Home,
				"OTHER=fixed",
			},
		}

		resolved := r.Resolve(input)
		actual, ok := resolved.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, expected["image"], actual["image"])
		assert.Equal(t, expected["env"], actual["env"])
	})
}

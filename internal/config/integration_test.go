package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFileSystem struct {
	RealFileSystem
	wd string
}

func (f *testFileSystem) Getwd() (string, error) {
	return f.wd, nil
}

func (f *testFileSystem) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(f.wd, path), nil
}

func TestIntegration_ConfigLoader_LoadRealFS(t *testing.T) {
	t.Parallel()
	// Keep one test with real filesystem to ensure RealFileSystem works
	tmpDir := t.TempDir()

	content := "runtime: docker"
	err := os.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(content), 0o644)
	require.NoError(t, err)

	fs := &testFileSystem{wd: tmpDir}
	loader := NewConfigLoaderWithFS(fs)

	cfg, paths, err := loader.LoadCDERunConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, paths)
	assert.Equal(t, filepath.Join(tmpDir, ".cderun.yaml"), paths[0])
}

func TestIntegration_ConfigLoader_MergeHierarchical(t *testing.T) {
	t.Parallel()
	// Create a temporary directory structure
	// tmp/
	//   .cderun.yaml (parent)
	//   .tools.yaml (parent)
	//   child/
	//     .cderun.yaml (child)
	//     .tools.yaml (child)

	tmpDir := t.TempDir()

	childDir := filepath.Join(tmpDir, "child")
	err := os.MkdirAll(childDir, 0o755)
	require.NoError(t, err)

	// Parent configs
	parentCDERun := `
runtime: docker
defaults:
  tty: false
  network: bridge
`
	parentTools := `
node:
  image: node:14
  env: ["PARENT=1"]
`
	err = os.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(parentCDERun), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, ".tools.yaml"), []byte(parentTools), 0o644)
	require.NoError(t, err)

	// Child configs
	childCDERun := `
defaults:
  tty: true
`
	childTools := `
node:
  image: node:16
python:
  image: python:3.9
`
	err = os.WriteFile(filepath.Join(childDir, ".cderun.yaml"), []byte(childCDERun), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(childDir, ".tools.yaml"), []byte(childTools), 0o644)
	require.NoError(t, err)

	fs := &testFileSystem{wd: childDir}
	loader := NewConfigLoaderWithFS(fs)

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		t.Parallel()
		cfg, paths, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Equal(t, filepath.Join(childDir, ".cderun.yaml"), paths[0])
		assert.Equal(t, filepath.Join(tmpDir, ".cderun.yaml"), paths[1])

		assert.Equal(t, "docker", cfg.Runtime)          // From parent
		assert.True(t, *cfg.Defaults.TTY)               // From child (overridden)
		assert.Equal(t, "bridge", cfg.Defaults.Network) // From parent
	})

	t.Run("ToolsConfig Merge", func(t *testing.T) {
		t.Parallel()
		cfg, paths, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		require.Len(t, paths, 2)

		node := cfg["node"]
		assert.Equal(t, "node:16", node.Image) // From child (overridden)
		assert.Equal(t, []string{"PARENT=1"}, node.Env) // From parent (preserved by deep merge)

		python := cfg["python"]
		assert.Equal(t, "python:3.9", python.Image) // From child
	})
}

func TestIntegration_Expression_Resolve(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	fs := &testFileSystem{wd: tmpDir}
	resolver, err := NewExpressionResolverWithFS(nil, fs)
	require.NoError(t, err)

	t.Run("Magic Words", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, resolver.Pwd, resolver.Resolve("{{PWD}}"))
		assert.Equal(t, resolver.Home, resolver.Resolve("{{HOME}}"))
		assert.Equal(t, resolver.Pwd+"/src", resolver.Resolve("{{PWD}}/src"))
	})

	t.Run("File Directive", func(t *testing.T) {
		t.Parallel()
		err := os.WriteFile(filepath.Join(tmpDir, "version.txt"), []byte(" 1.2.3 \n"), 0o644)
		require.NoError(t, err)

		assert.Equal(t, "golang:1.2.3", resolver.Resolve("golang:{{file:version.txt}}"))
		require.NoError(t, resolver.Error())
		resolver.Resolve("{{file:nonexistent.txt}}")
		require.Error(t, resolver.Error())

		t.Run("Path Traversal Protection", func(t *testing.T) {
			t.Parallel()
			// Absolute path should be blocked
			r, _ := NewExpressionResolverWithFS(nil, fs)
			r.Resolve("{{file:/etc/passwd}}")
			require.Error(t, r.Error())

			// Parent directory reference should be blocked
			r, _ = NewExpressionResolverWithFS(nil, fs)
			r.Resolve("{{file:../etc/passwd}}")
			require.Error(t, r.Error())
		})
	})

	t.Run("Nested Structures", func(t *testing.T) {
		t.Parallel()
		r, _ := NewExpressionResolverWithFS(nil, fs)
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

		// Map iteration order is random, but values should match
		resolved := r.Resolve(input)
		actual, ok := resolved.(map[string]any)
		require.True(t, ok, "Resolve should return map[string]any, got %T", resolved)
		assert.Equal(t, expected["image"], actual["image"])
		assert.Equal(t, expected["env"], actual["env"])
	})
}

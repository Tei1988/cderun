package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_Config_Load_RealFS(t *testing.T) {
	// Keep one test with real filesystem to ensure RealFileSystem works
	tmpDir, err := os.MkdirTemp("", "cderun-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := "runtime: docker"
	err = os.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	// Changing the working directory is process-global and can affect parallel tests.
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		// Restore the original working directory after the test.
		require.NoError(t, os.Chdir(originalWd))
	})

	cfg, paths, err := LoadCDERunConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, paths)
}

func TestIntegration_Config_Merge_Hierarchical(t *testing.T) {
	// Create a temporary directory structure
	// tmp/
	//   .cderun.yaml (parent)
	//   .tools.yaml (parent)
	//   child/
	//     .cderun.yaml (child)
	//     .tools.yaml (child)

	tmpDir, err := os.MkdirTemp("", "cderun-merge-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	childDir := filepath.Join(tmpDir, "child")
	err = os.MkdirAll(childDir, 0755)
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
	err = os.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(parentCDERun), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, ".tools.yaml"), []byte(parentTools), 0644)
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
	err = os.WriteFile(filepath.Join(childDir, ".cderun.yaml"), []byte(childCDERun), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(childDir, ".tools.yaml"), []byte(childTools), 0644)
	require.NoError(t, err)

	// Change working directory to childDir
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(childDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalWd)) })

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Contains(t, paths[0], filepath.Join("child", ".cderun.yaml"))
		assert.Contains(t, paths[1], filepath.Join(".cderun.yaml"))

		assert.Equal(t, "docker", cfg.Runtime)          // From parent
		assert.True(t, *cfg.Defaults.TTY)               // From child (overridden)
		assert.Equal(t, "bridge", cfg.Defaults.Network) // From parent
	})

	t.Run("ToolsConfig Merge", func(t *testing.T) {
		cfg, paths, err := LoadToolsConfig()
		assert.NoError(t, err)
		require.Len(t, paths, 2)

		node := cfg["node"]
		assert.Equal(t, "node:16", node.Image) // From child (overridden)
		// Note: 既存の ToolConfig に対して mergo.Merge を使って深いマージを行う (LoadToolsConfig 内)。
		// yaml.Unmarshal はファイルの読み込みに使用され、実際の深いマージは mergo.Merge が担当する。
		assert.Equal(t, []string{"PARENT=1"}, node.Env) // From parent (preserved by deep merge)

		python := cfg["python"]
		assert.Equal(t, "python:3.9", python.Image) // From child
	})
}

func TestIntegration_Config_Expression_Resolve(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cderun-expr-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalWd)) })

	resolver, err := NewExpressionResolver(nil)
	require.NoError(t, err)

	t.Run("Magic Words", func(t *testing.T) {
		assert.Equal(t, resolver.Pwd, resolver.Resolve("{{PWD}}"))
		assert.Equal(t, resolver.Home, resolver.Resolve("{{HOME}}"))
		assert.Equal(t, resolver.Pwd+"/src", resolver.Resolve("{{PWD}}/src"))
	})

	t.Run("File Directive", func(t *testing.T) {
		err := os.WriteFile("version.txt", []byte(" 1.2.3 \n"), 0644)
		require.NoError(t, err)

		assert.Equal(t, "golang:1.2.3", resolver.Resolve("golang:{{file:version.txt}}"))
		assert.Equal(t, "", resolver.Resolve("{{file:nonexistent.txt}}"))
	})

	t.Run("Nested Structures", func(t *testing.T) {
		input := map[string]any{
			"image": "node:{{PWD}}",
			"env": []any{
				"HOME={{HOME}}",
				"OTHER=fixed",
			},
		}
		expected := map[string]any{
			"image": "node:" + resolver.Pwd,
			"env": []any{
				"HOME=" + resolver.Home,
				"OTHER=fixed",
			},
		}

		// Map iteration order is random, but values should match
		resolved := resolver.Resolve(input)
		actual, ok := resolved.(map[string]any)
		require.True(t, ok, "Resolve should return map[string]any, got %T", resolved)
		assert.Equal(t, expected["image"], actual["image"])
		assert.Equal(t, expected["env"], actual["env"])
	})
}

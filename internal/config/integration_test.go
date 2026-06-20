package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_Config_Load_MockFS(t *testing.T) {
	t.Parallel()
	tmpDir := "/tmp/cderun-test"
	content := "runtime: docker"

	mfs := &MockFileSystem{
		WD: tmpDir,
		Files: map[string][]byte{
			filepath.Join(tmpDir, ".cderun.yaml"): []byte(content)}}
	loader := NewConfigLoaderWithFS(mfs)
	cfg, paths, err := loader.LoadCDERunConfig()

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, paths)
	assert.Equal(t, "docker", cfg.Runtime)
}

func TestIntegration_Config_Merge_Hierarchical(t *testing.T) {
	t.Parallel()
	// Given: A hierarchical directory structure with configs in parent and child
	// /tmp/
	//   .cderun.yaml (parent)
	//   .tools.yaml (parent)
	//   child/
	//     .cderun.yaml (child)
	//     .tools.yaml (child)

	parentDir := "/tmp"
	childDir := "/tmp/child"

	mfs := &MockFileSystem{
		WD: childDir,
		Dirs: map[string]bool{
			parentDir: true,
			childDir:  true},
		Files: map[string][]byte{
			filepath.Join(parentDir, ".cderun.yaml"): []byte("runtime: docker\ndefaults:\n  tty: false\n  network: bridge"),
			filepath.Join(parentDir, ".tools.yaml"):  []byte("node:\n  image: node:14\n  env: [\"PARENT=1\"]"),
			filepath.Join(childDir, ".cderun.yaml"):  []byte("defaults:\n  tty: true"),
			filepath.Join(childDir, ".tools.yaml"):   []byte("node:\n  image: node:16\npython:\n  image: python:3.9")}}
	loader := NewConfigLoaderWithFS(mfs)

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		// When: Loading CDERun config from child directory
		cfg, paths, err := loader.LoadCDERunConfig()

		// Then: Configs should be merged correctly (child overrides parent)
		require.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Equal(t, "docker", cfg.Runtime)          // From parent
		assert.True(t, *cfg.Defaults.TTY)               // From child (overridden)
		assert.Equal(t, "bridge", cfg.Defaults.Network) // From parent
	})

	t.Run("ToolsConfig Merge", func(t *testing.T) {
		// When: Loading tools config from child directory
		cfg, paths, err := loader.LoadToolsConfig()

		// Then: Tool configs should be merged correctly
		require.NoError(t, err)
		require.Len(t, paths, 2)

		node := cfg["node"]
		assert.Equal(t, "node:16", node.Image)          // From child (overridden)
		assert.Equal(t, []string{"PARENT=1"}, node.Env) // From parent (preserved)

		python := cfg["python"]
		assert.Equal(t, "python:3.9", python.Image) // From child
	})
}

func TestIntegration_Config_Expression_Resolve(t *testing.T) {
	t.Parallel()
	// Given: Expression resolver with MockFileSystem
	mfs := &MockFileSystem{
		WD:      "/app",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/app/version.txt": []byte(" 1.2.3 \n")}}
	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Magic Words", func(t *testing.T) {
		// When/Then: PWD and HOME are resolved
		assert.Equal(t, "/app", resolver.Resolve("{{PWD}}"))
		assert.Equal(t, "/home/user", resolver.Resolve("{{HOME}}"))
	})

	t.Run("File Directive", func(t *testing.T) {
		// When/Then: file directive reads from MockFileSystem
		assert.Equal(t, "golang:1.2.3", resolver.Resolve("golang:{{file:version.txt}}"))
		require.NoError(t, resolver.Error())

		resolver.Resolve("{{file:nonexistent.txt}}")
		require.Error(t, resolver.Error())
	})

	t.Run("Path Traversal Protection", func(t *testing.T) {
		// When/Then: Path traversal attempts are blocked
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		r.Resolve("{{file:/etc/passwd}}")
		require.Error(t, r.Error())

		r, err = NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		r.Resolve("{{file:../etc/passwd}}")
		require.Error(t, r.Error())
	})
}

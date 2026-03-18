package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_Config_Load_MockFS(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/.cderun.yaml": []byte("runtime: docker"),
		},
		Dirs: map[string]bool{"/project": true},
		WD:   "/project",
	}
	loader := NewConfigLoaderWithFS(mfs)

	cfg, paths, err := loader.LoadCDERunConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "/project/.cderun.yaml", paths[0])
	assert.Equal(t, "docker", cfg.Runtime)
}

func TestIntegration_Config_Merge_Hierarchical(t *testing.T) {
	t.Parallel()
	// Create a temporary directory structure in MockFileSystem
	// /tmp/
	//   .cderun.yaml (parent)
	//   .tools.yaml (parent)
	//   child/
	//     .cderun.yaml (child)
	//     .tools.yaml (child)

	mfs := &MockFileSystem{
		Files: map[string][]byte{
			"/tmp/.cderun.yaml": []byte(`
runtime: docker
defaults:
  tty: false
  network: bridge
`),
			"/tmp/.tools.yaml": []byte(`
node:
  image: node:14
  env: ["PARENT=1"]
`),
			"/tmp/child/.cderun.yaml": []byte(`
defaults:
  tty: true
`),
			"/tmp/child/.tools.yaml": []byte(`
node:
  image: node:16
python:
  image: python:3.9
`),
		},
		Dirs: map[string]bool{
			"/tmp":       true,
			"/tmp/child": true,
		},
		WD: "/tmp/child",
	}
	loader := NewConfigLoaderWithFS(mfs)

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		cfg, paths, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Equal(t, "/tmp/child/.cderun.yaml", paths[0])
		assert.Equal(t, "/tmp/.cderun.yaml", paths[1])

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
		Dirs: map[string]bool{"/project": true},
	}

	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Magic Words", func(t *testing.T) {
		assert.Equal(t, "/project", resolver.Resolve("{{PWD}}"))
		assert.Equal(t, "/home/user", resolver.Resolve("{{HOME}}"))
		assert.Equal(t, "/project/src", resolver.Resolve("{{PWD}}/src"))
	})

	t.Run("File Directive", func(t *testing.T) {
		assert.Equal(t, "golang:1.2.3", resolver.Resolve("golang:{{file:version.txt}}"))
		require.NoError(t, resolver.Error())

		res := resolver.Resolve("{{file:nonexistent.txt}}")
		assert.Equal(t, "{{file:nonexistent.txt}}", res)
		require.Error(t, resolver.Error())
		resolver, _ = NewExpressionResolverWithFS(nil, mfs) // Reset

		t.Run("Path Traversal Protection", func(t *testing.T) {
			// Absolute path should be blocked
			resolver, _ = NewExpressionResolverWithFS(nil, mfs)
			resolver.Resolve("{{file:/etc/passwd}}")
			require.Error(t, resolver.Error())

			// Parent directory reference should be blocked
			resolver, _ = NewExpressionResolverWithFS(nil, mfs)
			resolver.Resolve("{{file:../etc/passwd}}")
			require.Error(t, resolver.Error())
		})
	})

	t.Run("Nested Structures", func(t *testing.T) {
		resolver, _ = NewExpressionResolverWithFS(nil, mfs) // Reset
		input := map[string]any{
			"image": "node:{{PWD}}",
			"env": []any{
				"HOME={{HOME}}",
				"OTHER=fixed",
			},
		}
		expected := map[string]any{
			"image": "node:/project",
			"env": []any{
				"HOME=/home/user",
				"OTHER=fixed",
			},
		}

		resolved := resolver.Resolve(input)
		actual, ok := resolved.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, expected["image"], actual["image"])
		assert.Equal(t, expected["env"], actual["env"])
	})
}

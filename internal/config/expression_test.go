package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpressionResolver(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cderun-expr-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldWd) }()

	resolver, err := NewExpressionResolver()
	require.NoError(t, err)

	t.Run("Magic Words", func(t *testing.T) {
		assert.Equal(t, resolver.pwd, resolver.Resolve("{{PWD}}"))
		assert.Equal(t, resolver.home, resolver.Resolve("{{HOME}}"))
		assert.Equal(t, resolver.pwd+"/src", resolver.Resolve("{{PWD}}/src"))
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
			"image": "node:" + resolver.pwd,
			"env": []any{
				"HOME=" + resolver.home,
				"OTHER=fixed",
			},
		}

		// Map iteration order is random, but values should match
		actual := resolver.Resolve(input).(map[string]any)
		assert.Equal(t, expected["image"], actual["image"])
		assert.Equal(t, expected["env"], actual["env"])
	})
}

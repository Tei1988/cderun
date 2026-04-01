package command

import (
	"testing"

	"cderun/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_PreprocessArgs_BoolFlagConsumesNext(t *testing.T) {
	o := &rootOptions{}
	cmd := newRootCmd(o)

	// cderun node --cderun-tty app.js
	// expected: ["cderun", "--cderun-tty", "node", "app.js"]
	args := []string{"cderun", "node", "--cderun-tty", "app.js"}

	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	assert.Equal(t, []string{"cderun", "--cderun-tty", "node", "app.js"}, processed)
}

func TestSecurity_PreprocessArgs_P1DoesNotConsumeP1(t *testing.T) {
	o := &rootOptions{}
	cmd := newRootCmd(o)

	// cderun node --cderun-image alpine --cderun-tty
	// expected: ["cderun", "--cderun-image", "alpine", "--cderun-tty", "node"]
	args := []string{"cderun", "node", "--cderun-image", "alpine", "--cderun-tty"}

	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	assert.Equal(t, []string{"cderun", "--cderun-image", "alpine", "--cderun-tty", "node"}, processed)
}

func TestSecurity_SubcommandDetection_PersistentFlag(t *testing.T) {
	o := &rootOptions{}
	cmd := newRootCmd(o)

	// --image is a persistent flag.
	args := []string{"cderun", "--image", "alpine", "sh"}
	// Expected subcmdIdx = 3 (sh)

	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	// If fix is correct, processed[3] will be "sh"
	assert.Equal(t, "sh", processed[3], "Should correctly detect 'sh' as subcommand even after persistent flag with argument")
}

func TestSecurity_PreprocessArgs_ShorthandCluster(t *testing.T) {
	o := &rootOptions{}
	cmd := newRootCmd(o)

	// cderun -it node
	// The cluster -it contains only boolean flags, so 'node' should be detected as subcommand at index 2.
	args := []string{"cderun", "-it", "node"}
	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, []string{"cderun", "-it", "node"}, processed)
}

func TestSecurity_PreprocessArgs_P1BeforeSubcommand_Bypass(t *testing.T) {
	o := &rootOptions{}
	cmd := newRootCmd(o)

	// This should error because --cderun-tty is before 'node'
	args := []string{"cderun", "--image", "alpine", "--cderun-tty", "node"}

	_, err := preprocessArgs(cmd, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be placed after the subcommand")
}

func TestSecurity_ResolveFile_LargeFile(t *testing.T) {
	mfs := &config.MockFileSystem{
		Files: map[string][]byte{
			"/huge.txt": make([]byte, config.MaxDirectiveFileSize+1),
		},
		WD: "/",
	}
	r, err := config.NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	_, err = r.ResolveString("{{file:huge.txt}}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestSecurity_ResolveFile_CacheError(t *testing.T) {
	mfs := &config.MockFileSystem{
		Files: map[string][]byte{
			"/too-large.txt": make([]byte, config.MaxDirectiveFileSize+1),
		},
		WD: "/",
	}
	r, err := config.NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	_, err = r.ResolveString("{{file:too-large.txt}}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")

	// Call again, should still fail with same error from cache
	_, err = r.ResolveString("{{file:too-large.txt}}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestSecurity_MaskSensitiveEnv(t *testing.T) {
	env := []string{
		"NORMAL=value",
		"MY_SECRET=hidden",
		"API_KEY=12345",
		"TOKEN_VAR=abcde",
		"USER_PASSWORD=pass",
		"NOVALUE",
	}

	masked := config.MaskSensitiveEnv(env)

	assert.Equal(t, "NORMAL=value", masked[0])
	assert.Equal(t, "MY_SECRET=[REDACTED]", masked[1])
	assert.Equal(t, "API_KEY=[REDACTED]", masked[2])
	assert.Equal(t, "TOKEN_VAR=[REDACTED]", masked[3])
	assert.Equal(t, "USER_PASSWORD=[REDACTED]", masked[4])
	assert.Equal(t, "NOVALUE", masked[5])
}

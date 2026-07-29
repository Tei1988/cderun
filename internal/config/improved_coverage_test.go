package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ExpressionResolver_ResolveFile_MaxDirectiveFileSizeBoundary(t *testing.T) {
	hostCtx := &HostContext{}

	t.Run("resolveFile exactly MaxDirectiveFileSize", func(t *testing.T) {
		content := make([]byte, MaxDirectiveFileSize)
		for i := range content {
			content[i] = 'a'
		}
		fs := &MockFileSystem{
			Files: map[string][]byte{"/project/limit.txt": content},
			WD:    "/project",
		}
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.resolveFile("limit.txt")
		require.NoError(t, err)
		assert.Len(t, val, MaxDirectiveFileSize)
	})

	t.Run("resolveFile exceeding MaxDirectiveFileSize in Stat", func(t *testing.T) {
		fs := &MockFileSystem{
			Files: map[string][]byte{"/project/large.txt": make([]byte, MaxDirectiveFileSize+1)},
			WD:    "/project",
		}
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		_, err = r.resolveFile("large.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})

	t.Run("resolveFile exceeding MaxDirectiveFileSize in ReadFile", func(t *testing.T) {
		fs := &MockFileSystem{
			Files: map[string][]byte{"/project/race.txt": make([]byte, MaxDirectiveFileSize+1)},
			WD:    "/project",
		}
		cfs := &customStatFS{
			MockFileSystem: *fs,
			statSize:       10,
		}

		r, err := NewExpressionResolverWithFS(hostCtx, cfs)
		require.NoError(t, err)

		_, err = r.resolveFile("race.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})

	t.Run("resolveFile error caching", func(t *testing.T) {
		fs := &exprMockFS{
			MockFileSystem: MockFileSystem{WD: "/project"},
			readFileErr:    assert.AnError,
		}
		fs.Files = map[string][]byte{"/project/err.txt": []byte("data")}

		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		_, err = r.resolveFile("err.txt")
		require.Error(t, err)

		_, err2 := r.resolveFile("err.txt")
		assert.Same(t, err, err2)
	})
}

type customStatFS struct {
	MockFileSystem
	statSize int64
}

func (f *customStatFS) Stat(name string) (os.FileInfo, error) {
	info, err := f.MockFileSystem.Stat(name)
	if err != nil {
		return nil, err
	}
	return &customStatInfo{FileInfo: info, size: f.statSize}, nil
}

type customStatInfo struct {
	os.FileInfo
	size int64
}

func (i *customStatInfo) Size() int64 { return i.size }

func TestUnit_OptionRegistry_Getters(t *testing.T) {
	t.Run("GetIntOption", func(t *testing.T) {
		opt, ok := GetIntOption("pull-max-retries")
		assert.True(t, ok)
		assert.Equal(t, "pull-max-retries", opt.Name)

		_, ok = GetIntOption("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetFloat64Option", func(t *testing.T) {
		opt, ok := GetFloat64Option("cpus")
		assert.True(t, ok)
		assert.Equal(t, "cpus", opt.Name)

		_, ok = GetFloat64Option("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetStringSliceOption", func(t *testing.T) {
		opt, ok := GetStringSliceOption("env")
		assert.True(t, ok)
		assert.Equal(t, "env", opt.Name)

		_, ok = GetStringSliceOption("nonexistent")
		assert.False(t, ok)
	})
}

func TestUnit_Config_SetBaseDirBehavior(t *testing.T) {
	t.Run("CDERunConfig.SetBaseDir with HostContext Mounts", func(t *testing.T) {
		cfg := &CDERunConfig{
			HostContext: &HostContext{
				Mounts: []MountMapping{
					{Source: "./src", Target: "/tgt"},
				},
			},
		}
		err := cfg.SetBaseDir("/base")
		require.NoError(t, err)
		assert.Equal(t, "/base/src", cfg.HostContext.Mounts[0].Source)
	})

	t.Run("ToolConfig.SetBaseDir", func(t *testing.T) {
		tc := &ToolConfig{
			Mounts: []MountConfig{
				{Source: ConfigPath{Raw: "./s"}, Target: ConfigPath{Raw: "/t"}},
			},
		}
		tc.SetBaseDir("/base")
		assert.Equal(t, "/base", tc.Mounts[0].Source.BaseDir)
	})
}

func TestUnit_Resolver_InternalHelpers(t *testing.T) {
	t.Run("ptr helper", func(t *testing.T) {
		v := 123
		p := ptr(v)
		assert.Equal(t, v, *p)
	})

	t.Run("getFieldInfo for non-slice types", func(t *testing.T) {
		cli := &CLIOptions{TTY: ptr(true)}
		cliVal := reflect.ValueOf(cli).Elem()

		fieldOnce.Do(initFieldInfo)
		info, ok := fieldInfo["tty"]
		assert.True(t, ok, "fieldInfo['tty'] must exist")

		set, val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)
		assert.True(t, set)
		assert.True(t, val.Bool())
	})
}

func TestUnit_ExpressionResolver_ResolveString_Empty(t *testing.T) {
	r, err := NewExpressionResolver(nil)
	require.NoError(t, err)
	assert.Empty(t, r.resolveString(""))
}

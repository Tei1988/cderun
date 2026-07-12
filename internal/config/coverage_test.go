package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Coverage_Option_ParsingErrors(t *testing.T) {
	mfs := &MockFileSystem{Env: map[string]string{
		"I": "bad",
		"F": "bad",
		"B": "bad",
	}}
	_, errI := resolveIntOpt(OptionDef[*int]{EnvKey: "I", Fallback: ptr(5)}, false, 0, false, 0, "s", nil, nil, mfs)
	assert.Error(t, errI)

	_, errF := resolveFloat64Opt(OptionDef[*float64]{EnvKey: "F", Fallback: ptr(1.1)}, false, 0, false, 0, "s", nil, nil, mfs)
	assert.Error(t, errF)

	_, _, errB := resolveBoolOptInfo(OptionDef[*bool]{EnvKey: "B"}, false, false, false, false, "s", nil, nil, mfs)
	assert.Error(t, errB)
}

func TestUnit_Coverage_Config_FindConfigs_AbsError(t *testing.T) {
	mfs := &MockFileSystem{
		WD:      "/app",
		HomeDir: "/home/user",
		Dirs: map[string]bool{
			"/app/.cderun.yaml":                      true,
			"/home/user/.config/cderun/.cderun.yaml": true,
		},
		AbsErr: assert.AnError,
	}
	loader := NewConfigLoaderWithFS(mfs)
	paths := loader.FindConfigs(".cderun.yaml")
	assert.NotEmpty(t, paths)
}

func TestUnit_Coverage_Option_IntOpt_Fallback(t *testing.T) {
	def := OptionDef[*int]{Fallback: ptr(42)}
	res, _ := resolveIntOpt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.Equal(t, 42, res)
	def.Fallback = nil
	res, _ = resolveIntOpt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.Equal(t, 0, res)
}

func TestUnit_Coverage_Option_Float64Opt_Fallback(t *testing.T) {
	def := OptionDef[*float64]{Fallback: ptr(3.14)}
	res, _ := resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.InDelta(t, 3.14, res, 1e-9)
	def.Fallback = nil
	res, _ = resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.InDelta(t, 0.0, res, 1e-9)
}

func TestUnit_Coverage_Option_BoolOpt_EnvKeyEmpty(t *testing.T) {
	def := OptionDef[*bool]{EnvKey: ""}
	_, spec, err := resolveBoolOptInfo(def, false, false, false, false, "s", nil, nil, &MockFileSystem{})
	assert.NoError(t, err)
	assert.False(t, spec)
}

func TestUnit_Coverage_Option_StringSlice_EmptyEnv(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	res := resolveStringSliceOpt(def, ",", nil, nil, "s", nil, nil, nil, &MockFileSystem{Env: map[string]string{"E": ""}})
	assert.Empty(t, res)
}

func TestUnit_Coverage_Option_StringSlice_NoEnv(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	res := resolveStringSliceOpt(def, ",", nil, nil, "s", nil, nil, nil, &MockFileSystem{})
	assert.Nil(t, res)
}

func TestUnit_Coverage_Option_StringSliceComma_NoEnv(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	res := resolveStringSliceCommaOpt(def, false, "", false, "", "s", nil, nil, nil, &MockFileSystem{})
	assert.Nil(t, res)
}

func TestUnit_Coverage_Config_LoadConfigs_NoPaths(t *testing.T) {
	mfs := &MockFileSystem{}
	loader := NewConfigLoaderWithFS(mfs)
	cfg, paths, err := loader.LoadCDERunConfig()
	assert.NoError(t, err)
	assert.Nil(t, cfg)
	assert.Nil(t, paths)

	tCfg, tPaths, err := loader.LoadToolsConfig()
	assert.NoError(t, err)
	assert.Nil(t, tCfg)
	assert.Nil(t, tPaths)
}

func TestUnit_Coverage_Config_LoadCDERunConfig_ReadError(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/app",
		Dirs: map[string]bool{
			"/app/.cderun.yaml": true,
		},
		ReadFileErr: os.ErrPermission,
	}
	loader := NewConfigLoaderWithFS(mfs)
	_, _, err := loader.LoadCDERunConfig()
	assert.Error(t, err)
}

func TestUnit_Coverage_Config_LoadToolsConfig_ReadError(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/app",
		Dirs: map[string]bool{
			"/app/.tools.yaml": true,
		},
		ReadFileErr: os.ErrPermission,
	}
	loader := NewConfigLoaderWithFS(mfs)
	_, _, err := loader.LoadToolsConfig()
	assert.Error(t, err)
}

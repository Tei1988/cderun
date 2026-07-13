package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type CoverageErrorFS struct {
	*MockFileSystem
	WDErr   error
	HomeErr error
}

func (f *CoverageErrorFS) Getwd() (string, error) {
	if f.WDErr != nil {
		return "", f.WDErr
	}
	return f.MockFileSystem.Getwd()
}

func (f *CoverageErrorFS) UserHomeDir() (string, error) {
	if f.HomeErr != nil {
		return "", f.HomeErr
	}
	return f.MockFileSystem.UserHomeDir()
}

func TestUnit_Coverage_PascalCase_Exhaustive(t *testing.T) {
	assert.Equal(t, "MyOption", PascalCase("my--option"))
	assert.Equal(t, "Option", PascalCase("-option-"))
	assert.Empty(t, PascalCase("--"))
	assert.Equal(t, "TTYOption", PascalCase("tty-option"))
	assert.Equal(t, "OptionTTY", PascalCase("option-tty"))
	assert.Equal(t, "CPUsOption", PascalCase("cpus-option"))
}

func TestUnit_Coverage_Option_ParsingErrors(t *testing.T) {
	mfs := &MockFileSystem{Env: map[string]string{
		"I": "bad",
		"F": "bad",
		"B": "bad",
	}}
	_, err := resolveIntOpt(OptionDef[*int]{EnvKey: "I", Fallback: ptr(5)}, false, 0, false, 0, "s", nil, nil, mfs)
	assert.Error(t, err)

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
		AbsErr: errors.New("abs error"),
	}
	l := NewConfigLoaderWithFS(mfs)
	paths := l.FindConfigs(".cderun.yaml")
	assert.Contains(t, paths, "/app/.cderun.yaml")
	assert.Contains(t, paths, "/home/user/.config/cderun/.cderun.yaml")
}

func TestUnit_Coverage_Config_FindConfigs_Errors(t *testing.T) {
	mfs := &CoverageErrorFS{
		MockFileSystem: &MockFileSystem{
			HomeDir: "/home/user",
			Dirs: map[string]bool{
				"/home/user/.config/cderun/.cderun.yaml": true,
				"/etc/cderun/.cderun.yaml":               true,
				"/run/cderun/.cderun.yaml":               true,
			},
		},
		WDErr: errors.New("getwd failed"),
	}
	l := NewConfigLoaderWithFS(mfs)
	paths := l.FindConfigs(".cderun.yaml")
	assert.NotEmpty(t, paths)
	assert.Contains(t, paths, "/home/user/.config/cderun/.cderun.yaml")

	mfs = &CoverageErrorFS{
		MockFileSystem: &MockFileSystem{
			WD: "/app",
			Dirs: map[string]bool{
				"/app/.cderun.yaml":        true,
				"/etc/cderun/.cderun.yaml": true,
				"/run/cderun/.cderun.yaml": true,
			},
		},
		HomeErr: errors.New("home error"),
	}
	l = NewConfigLoaderWithFS(mfs)
	paths = l.FindConfigs(".cderun.yaml")
	assert.NotEmpty(t, paths)
	assert.Contains(t, paths, "/app/.cderun.yaml")
}

func TestUnit_Coverage_Config_LoadConfigs_NoPaths(t *testing.T) {
	mfs := &MockFileSystem{}
	loader := NewConfigLoaderWithFS(mfs)
	cfg, paths, err := loader.LoadCDERunConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
	assert.Nil(t, paths)

	tCfg, tPaths, err := loader.LoadToolsConfig()
	require.NoError(t, err)
	assert.Nil(t, tCfg)
	assert.Nil(t, tPaths)
}

func TestUnit_Coverage_Option_IntOpt_Fallback(t *testing.T) {
	def := OptionDef[*int]{Fallback: ptr(42)}
	res, err := resolveIntOpt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.NoError(t, err)
	assert.Equal(t, 42, res)
	def.Fallback = nil
	res, err = resolveIntOpt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.NoError(t, err)
	assert.Equal(t, 0, res)
}

func TestUnit_Coverage_Option_Float64Opt_Fallback(t *testing.T) {
	def := OptionDef[*float64]{Fallback: ptr(3.14)}
	res, err := resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, res, 1e-9)
	def.Fallback = nil
	res, err = resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{})
	assert.NoError(t, err)
	assert.InDelta(t, 0.0, res, 1e-9)
}

func TestUnit_Coverage_Option_BoolOpt_EnvKeyEmpty(t *testing.T) {
	def := OptionDef[*bool]{EnvKey: ""}
	val, spec, errB := resolveBoolOptInfo(def, false, false, false, false, "s", nil, nil, &MockFileSystem{})
	assert.NoError(t, errB)
	assert.False(t, spec)
	assert.False(t, val)
}

func TestUnit_Coverage_Option_StringSlice_EmptyEnv(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	r, _ := NewExpressionResolver(nil)
	mfs := &MockFileSystem{Env: map[string]string{"E": ""}}
	res := resolveStringSliceOpt(def, ",", nil, nil, "s", nil, nil, r, mfs)
	assert.Empty(t, res)
}

func TestUnit_Coverage_Option_StringSlice_NoEnv(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	r, _ := NewExpressionResolver(nil)
	res := resolveStringSliceOpt(def, ",", nil, nil, "s", nil, nil, r, &MockFileSystem{})
	assert.Nil(t, res)
}

func TestUnit_Coverage_Option_StringSliceComma_NoEnv(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	r, _ := NewExpressionResolver(nil)
	res := resolveStringSliceCommaOpt(def, false, "", false, "", "s", nil, nil, r, &MockFileSystem{})
	assert.Nil(t, res)
}

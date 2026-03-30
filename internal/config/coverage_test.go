package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
	assert.Equal(t, "", PascalCase("--"))
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
	assert.Equal(t, 5, resolveIntOpt(OptionDef[*int]{EnvKey: "I", Fallback: ptr(5)}, false, 0, false, 0, "s", nil, nil, mfs))
	assert.InDelta(t, 1.1, resolveFloat64Opt(OptionDef[*float64]{EnvKey: "F", Fallback: ptr(1.1)}, false, 0, false, 0, "s", nil, nil, mfs), 1e-9)
	_, spec := resolveBoolOptInfo(OptionDef[*bool]{EnvKey: "B"}, false, false, false, false, "s", nil, nil, mfs)
	assert.False(t, spec)
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
	mfs := &CoverageErrorFS{MockFileSystem: &MockFileSystem{}, WDErr: errors.New("getwd failed")}
	l := NewConfigLoaderWithFS(mfs)
	assert.Empty(t, l.FindConfigs(".cderun.yaml"))

	mfs = &CoverageErrorFS{MockFileSystem: &MockFileSystem{}, HomeErr: errors.New("home error")}
	l = NewConfigLoaderWithFS(mfs)
	paths := l.FindConfigs(".cderun.yaml")
	for _, p := range paths {
		assert.NotContains(t, p, ".config")
	}
}

func TestUnit_Coverage_Config_LoadFromPath_Errors(t *testing.T) {
	mfs := &CoverageErrorFS{MockFileSystem: &MockFileSystem{WD: "/"}}
	l := NewConfigLoaderWithFS(mfs)

	mfs.HomeErr = errors.New("home error")
	_, _, err := l.LoadCDERunConfigFromPath("~/config.yaml")
	require.Error(t, err)

	mfs.HomeErr = nil
	mfs.AbsErr = errors.New("abs error")
	_, _, err = l.LoadCDERunConfigFromPath("config.yaml")
	require.Error(t, err)

	mfs.AbsErr = nil
	mfs.ReadFileErr = errors.New("read error")
	_, _, err = l.LoadCDERunConfigFromPath("/config.yaml")
	require.Error(t, err)

	mfs.ReadFileErr = nil
	mfs.Files = map[string][]byte{"/config.yaml": []byte("invalid: [")}
	_, _, err = l.LoadCDERunConfigFromPath("/config.yaml")
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveWithFS_RegistryMismatch(t *testing.T) {
	originalOptions := StringOptions
	StringOptions = append(StringOptions, StringOption{Name: "not-in-cli", FieldName: "NonExistent"})
	StringOptions = append(StringOptions, StringOption{Name: "invalid-cli-mapping", FieldName: "TTY"})
	defer func() { StringOptions = originalOptions }()

	cli := CLIOptions{CderunImage: "alpine", CderunImageSet: true}
	_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch:")

	StringOptions = append(originalOptions, StringOption{Name: "hc", FieldName: "HostContext"})
	_, err = ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch: CLI reflection fields")
}

func TestUnit_Coverage_Resolver_Errors_DurationMemory(t *testing.T) {
	mfs := &MockFileSystem{}
	cli := CLIOptions{Image: "alpine", ImageSet: true, HangTimeout: "invalid", HangTimeoutSet: true}
	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)

	cli = CLIOptions{Image: "alpine", ImageSet: true, PullBackoffBase: "0s", PullBackoffBaseSet: true}
	_, err = ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)

	cli = CLIOptions{Image: "alpine", ImageSet: true, Memory: "invalid", MemorySet: true}
	_, err = ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)

	cli = CLIOptions{Image: "alpine", ImageSet: true, PullMaxRetries: 0, PullMaxRetriesSet: true}
	_, err = ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveWithFS_ExpressionError(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	cli := CLIOptions{Image: "{{file:missing}}", ImageSet: true}
	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveEnv_Error(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	r.setError(errors.New("err"))
	_, err := resolveEnv(nil, []string{"VAR=VAL"}, "E", "s", nil, nil, false, r, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveMounts_Error(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	r.setError(errors.New("err"))
	_, err := resolveMounts(nil, []string{"source=/s,target=/t"}, "s", nil, nil, r, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveDevices_Error(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	r.setError(errors.New("sticky error"))
	_, err := resolveDevices(nil, []string{"/dev/a:/dev/b"}, "s", nil, nil, r, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveConfigPath_Error(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	r.setError(errors.New("err"))
	_, err := resolveConfigPath(true, "val", false, "", "E", "s", nil, nil, nil, nil, "", r, "path", mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Path_UnmarshalYAML_Malformed(t *testing.T) {
	var mc MountConfig
	node := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "type"},
		{Kind: yaml.ScalarNode, Value: "bind"},
	}}
	err := mc.UnmarshalYAML(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mount target is required")

	nodeScalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "foo"}
	err = mc.UnmarshalYAML(nodeScalar)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mount config")

	var dc DeviceConfig
	err = dc.UnmarshalYAML(node)
	require.Error(t, err)

	var cp ConfigPath
	err = cp.UnmarshalYAML(node)
	require.Error(t, err)
}

func TestUnit_Coverage_Path_ParseDeviceConfig_Invalid(t *testing.T) {
	_, ok := ParseDeviceConfig("host:")
	assert.False(t, ok)
	_, ok = ParseDeviceConfig(":container")
	assert.False(t, ok)

	// Windows drive coverage
	h, r, ok := SplitHostRemainder("C:\\foo:container")
	assert.True(t, ok)
	assert.Equal(t, "C:\\foo", h)
	assert.Equal(t, "container", r)

	_, _, ok = SplitHostRemainder("C:\\foo")
	assert.False(t, ok)

	// ParseDeviceConfig with permissions
	dc, ok := ParseDeviceConfig("/dev/a:/dev/b:rw")
	assert.True(t, ok)
	assert.Equal(t, "rw", dc.Permissions)

	// Host without remainder
	dc, ok = ParseDeviceConfig("/dev/a")
	assert.True(t, ok)
	assert.Equal(t, "/dev/a", dc.Source.Raw)

	// Invalid host or container
	_, ok = ParseDeviceConfig(":")
	assert.False(t, ok)
	_, ok = ParseDeviceConfig("a:")
	assert.False(t, ok)
	_, ok = ParseDeviceConfig(":b")
	assert.False(t, ok)
}

func TestUnit_Coverage_Path_ParseMountFlag_Errors(t *testing.T) {
	_, err := ParseMountFlag("source=/s,target=/t,readonly=maybe")
	require.Error(t, err)
	_, err = ParseMountFlag("source=/s,target=/t,optional=maybe")
	require.Error(t, err)
}

func TestUnit_Coverage_Path_ResolvePath_Nested(t *testing.T) {
	hostCtx := &HostContext{
		Level: 1,
		Mounts: []MountMapping{
			{Source: "/host/a", Target: "/container/a", Level: 1},
		},
	}
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolverWithFS(hostCtx, mfs)

	res, err := ResolvePath("/container/a/file", "/base", r)
	require.NoError(t, err)
	assert.Equal(t, "/host/a/file", res)

	hostCtx.Mounts = append(hostCtx.Mounts, MountMapping{Source: "/host/a/b", Target: "/container/a/b", Level: 2})
	res, err = ResolvePath("/container/a/b/file", "/base", r)
	require.NoError(t, err)
	assert.Equal(t, "/host/a/b/file", res)

	res, err = ResolvePath("/other", "/base", r)
	require.NoError(t, err)
	assert.Equal(t, "/other", res)

	mfs.AbsErr = errors.New("abs err")
	_, err = ResolvePath("rel", "/base", r)
	require.Error(t, err)
}

func TestUnit_Coverage_Expression_ResolveString_Errors(t *testing.T) {
	mfs := &CoverageErrorFS{MockFileSystem: &MockFileSystem{}, HomeErr: errors.New("no home")}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	assert.Equal(t, "~/foo", r.resolveString("~/foo"))

	r.setError(nil)
	s := "{{file:missing}} {{HOME}}"
	r.resolveString(s)
	assert.Error(t, r.err)
}

func TestUnit_Coverage_Expression_Resolve_Complex(t *testing.T) {
	mfs := &MockFileSystem{HomeDir: "/home", WD: "/pwd"}
	r, _ := NewExpressionResolverWithFS(&HostContext{HomeDir: "/h"}, mfs)
	data := map[string]any{
		"a": "{{BASE_HOME}}",
		"b": []any{"~", "{{PWD}}"},
		"c": 123,
	}
	res := r.Resolve(data).(map[string]any)
	assert.Equal(t, "/h", res["a"])
	assert.Equal(t, "/home", res["b"].([]any)[0])
	assert.Equal(t, "/pwd", res["b"].([]any)[1])
	assert.Equal(t, 123, res["c"])

	r.setError(errors.New("err"))
	assert.Equal(t, "foo", r.Resolve("foo"))
}

func TestUnit_Coverage_Config_CachedStat_CacheHit(t *testing.T) {
	mfs := &MockFileSystem{Files: map[string][]byte{"f": {}}}
	l := NewConfigLoaderWithFS(mfs)
	_, _ = l.cachedStat("f")
	_, _ = l.cachedStat("f")
	assert.Len(t, mfs.StatCalls, 1)
}

func TestUnit_Coverage_MockFS_WriteFile_Error(t *testing.T) {
	mfs := &MockFileSystem{WriteFileErr: errors.New("write error")}
	err := mfs.WriteFile("f", []byte("d"), 0644)
	require.Error(t, err)

	mfs.WriteFileErr = nil
	err = mfs.WriteFile("f", []byte("d"), 0644)
	require.NoError(t, err)
}

func TestUnit_Coverage_MockFS_RemoveAll_Error(t *testing.T) {
	mfs := &MockFileSystem{RemoveAllErr: errors.New("remove error")}
	err := mfs.RemoveAll("f")
	require.Error(t, err)

	mfs.RemoveAllErr = nil
	mfs.Dirs = map[string]bool{"/d": true, "/d/f": true}
	mfs.Files = map[string][]byte{"/d/a": {}}
	err = mfs.RemoveAll("/d")
	require.NoError(t, err)
	assert.Empty(t, mfs.Dirs)
	assert.Empty(t, mfs.Files)

	mfs.Files = map[string][]byte{"/other": {}}
	err = mfs.RemoveAll("")
	require.NoError(t, err)
	assert.NotEmpty(t, mfs.Files)

	err = mfs.RemoveAll("/d")
	require.NoError(t, err)
	assert.NotEmpty(t, mfs.Files)
}

func TestUnit_Coverage_Config_Load_Errors(t *testing.T) {
	mfs := &MockFileSystem{WD: "/", Files: map[string][]byte{"/.cderun.yaml": []byte("r: d")}}
	l := NewConfigLoaderWithFS(mfs)
	mfs.ReadFileErr = errors.New("err")
	_, _, err := l.LoadCDERunConfig()
	require.Error(t, err)

	mfs.ReadFileErr = nil
	mfs.Files["/.cderun.yaml"] = []byte("invalid: [")
	_, _, err = l.LoadCDERunConfig()
	require.Error(t, err)

	mfs.Files["/.tools.yaml"] = []byte("n: { i: n }")
	mfs.ReadFileErr = errors.New("err")
	_, _, err = l.LoadToolsConfig()
	require.Error(t, err)

	mfs.ReadFileErr = nil
	mfs.Files = map[string][]byte{
		"/.tools.yaml":     []byte("node: { image: n1 }"),
		"/app/.tools.yaml": []byte("node: { pullMaxRetries: invalid }"),
	}
	mfs.WD = "/app"
	l = NewConfigLoaderWithFS(mfs)
	_, _, err = l.LoadToolsConfig()
	require.Error(t, err)
}

func TestUnit_Coverage_Config_DeepCopy_Exhaustive(t *testing.T) {
	d := ConfigDefaults{
		Devices: []DeviceConfig{{Source: ConfigPath{Raw: "/d"}}},
		Mounts:  []MountConfig{{Source: ConfigPath{Raw: "/s"}, Target: ConfigPath{Raw: "/t"}}},
	}
	cp := d.DeepCopy()
	assert.Len(t, cp.Devices, 1)
	assert.Len(t, cp.Mounts, 1)
}

func TestUnit_Coverage_Option_StringSlice_Priority(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	r, _ := NewExpressionResolver(nil)
	assert.Equal(t, []string{"ev"}, resolveStringSliceOpt(def, ",", nil, nil, "s", nil, nil, r, &MockFileSystem{Env: map[string]string{"E": "ev"}}))
	tools := ToolsConfig{"s": ToolConfig{Ports: []string{"80"}}}
	def.ToolGetter = func(t ToolConfig) []string { return t.Ports }
	assert.Equal(t, []string{"80"}, resolveStringSliceOpt(def, ",", nil, nil, "s", tools, nil, r, &MockFileSystem{}))
}

func TestUnit_Coverage_Resolver_ResolveEnv_EmptyParts(t *testing.T) {
	mfs := &MockFileSystem{Env: map[string]string{"E": "A=1; ;B=2"}}
	r, _ := NewExpressionResolver(nil)
	res, _ := resolveEnv(nil, nil, "E", "s", nil, nil, false, r, mfs)
	assert.Len(t, res, 2)
}

func TestUnit_Coverage_Resolver_ResolveMounts_EmptyParts(t *testing.T) {
	r, _ := NewExpressionResolver(nil)
	res, _ := resolveMounts(nil, nil, "s", nil, nil, r, &MockFileSystem{Env: map[string]string{"CDERUN_MOUNT": "source=/a,target=/b; ;source=/c,target=/d"}})
	assert.Len(t, res, 2)
}

func TestUnit_Coverage_Option_IntOpt_Fallback(t *testing.T) {
	def := OptionDef[*int]{Fallback: ptr(42)}
	assert.Equal(t, 42, resolveIntOpt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{}))
	def.Fallback = nil
	assert.Equal(t, 0, resolveIntOpt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{}))
}

func TestUnit_Coverage_Option_Float64Opt_Fallback(t *testing.T) {
	def := OptionDef[*float64]{Fallback: ptr(3.14)}
	assert.Equal(t, 3.14, resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{}))
	def.Fallback = nil
	assert.Equal(t, 0.0, resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{}))
}

func TestUnit_Coverage_Option_BoolOpt_EnvKeyEmpty(t *testing.T) {
	def := OptionDef[*bool]{EnvKey: ""}
	val, spec := resolveBoolOptInfo(def, false, false, false, false, "s", nil, nil, &MockFileSystem{})
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

func TestUnit_Coverage_Option_StringSliceComma_P1P2(t *testing.T) {
	def := OptionDef[[]string]{EnvKey: "E"}
	r, _ := NewExpressionResolver(nil)
	assert.Equal(t, []string{"a", "b"}, resolveStringSliceCommaOpt(def, true, "a,b", false, "", "s", nil, nil, r, &MockFileSystem{}))
	assert.Equal(t, []string{"c", "d"}, resolveStringSliceCommaOpt(def, false, "", true, "c,d", "s", nil, nil, r, &MockFileSystem{}))
	assert.Equal(t, []string{"e", "f"}, resolveStringSliceCommaOpt(def, false, "", false, "", "s", nil, nil, r, &MockFileSystem{Env: map[string]string{"E": "e,f"}}))
}

func TestUnit_Coverage_Resolver_InitFieldInfo_Coverage(t *testing.T) {
	fieldOnce.Do(initFieldInfo)
	assert.NotEmpty(t, fieldInfo)
	info, ok := fieldInfo["image"]
	assert.True(t, ok)
	assert.NotEmpty(t, info.targetIdx)
	assert.Equal(t, "TTY", PascalCase("tty"))
	assert.Equal(t, "DNS", PascalCase("dns"))
	assert.Equal(t, "CPUs", PascalCase("cpus"))
}

func TestUnit_Coverage_Config_Hierarchical_Load_NoConfigs(t *testing.T) {
	mfs := &MockFileSystem{}
	l := NewConfigLoaderWithFS(mfs)
	cfg, paths, err := l.LoadCDERunConfig()
	assert.NoError(t, err)
	assert.Nil(t, cfg)
	assert.Nil(t, paths)

	tcfg, tpaths, err := l.LoadToolsConfig()
	assert.NoError(t, err)
	assert.Nil(t, tcfg)
	assert.Nil(t, tpaths)
}

func TestUnit_Coverage_Config_NewConfigLoader(t *testing.T) {
	l := NewConfigLoader()
	assert.NotNil(t, l)
}

func TestUnit_Coverage_Expression_MagicWords_NoHostCtx(t *testing.T) {
	mfs := &MockFileSystem{HomeDir: "/h", WD: "/p"}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	assert.Equal(t, "/h", r.resolveString("{{BASE_HOME}}"))
	assert.Equal(t, "/p", r.resolveString("{{BASE_PWD}}"))
}

func TestUnit_Coverage_Expression_Resolve_MapSlice(t *testing.T) {
	r, _ := NewExpressionResolver(nil)
	data := []any{"{{HOME}}"}
	res := r.Resolve(data).([]any)
	assert.Len(t, res, 1)

	dataMap := map[string]any{"k": "{{HOME}}"}
	resMap := r.Resolve(dataMap).(map[string]any)
	assert.Contains(t, resMap, "k")
}

func TestUnit_Coverage_Config_SetDirs_Reset(t *testing.T) {
	r1 := SetRunConfigDirForTest("/tmp/run")
	r2 := SetSystemConfigDirForTest("/tmp/sys")
	assert.Equal(t, "/tmp/run", defaultLoader.runConfigDir)
	assert.Equal(t, "/tmp/sys", defaultLoader.systemConfigDir)
	r1()
	r2()
	assert.NotEqual(t, "/tmp/run", defaultLoader.runConfigDir)
}

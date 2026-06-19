package config

import (
	"errors"
	"path/filepath"
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
	// Case 1: WDErr should skip working directory search but still include others (home, system, run)
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
	assert.Contains(t, paths, "/etc/cderun/.cderun.yaml")
	assert.Contains(t, paths, "/run/cderun/.cderun.yaml")

	// Case 2: HomeErr should skip home directory search but include others (WD, system, run)
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
	assert.Contains(t, paths, "/etc/cderun/.cderun.yaml")
	assert.Contains(t, paths, "/run/cderun/.cderun.yaml")
	for _, p := range paths {
		assert.NotContains(t, p, "/home/user")
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
	_, err := ResolveWithFS("sh", &cli, nil, nil, &MockFileSystem{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch:")

	StringOptions = append(originalOptions, StringOption{Name: "hc", FieldName: "HostContext"})
	_, err = ResolveWithFS("sh", &cli, nil, nil, &MockFileSystem{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch: info for option \"hc\" not found")

	// Reset StringOptions before early resolution test to avoid triggering same error again
	StringOptions = originalOptions
	// Early resolution mismatch
	originalBool := BoolOptions
	BoolOptions = append(BoolOptions, BoolOption{Name: "diagnosis", FieldName: "NonExistent"})
	defer func() { BoolOptions = originalBool }()
	_, err = ResolveWithFS("sh", &cli, nil, nil, &MockFileSystem{})
	require.NoError(t, err) // It continues if mismatch in resolveEarly (currently)

	// Remaining bool mismatch
	BoolOptions = append(originalBool, BoolOption{Name: "bad-bool", FieldName: "NonExistent"})
	_, err = ResolveWithFS("sh", &cli, nil, nil, &MockFileSystem{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch: info for option \"bad-bool\" not found")
}

func TestUnit_Coverage_Resolver_Errors_DurationMemory(t *testing.T) {
	mfs := &MockFileSystem{}
	cli := CLIOptions{Image: "alpine", ImageSet: true, HangTimeout: "invalid", HangTimeoutSet: true}
	_, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)

	cli = CLIOptions{Image: "alpine", ImageSet: true, PullBackoffBase: "0s", PullBackoffBaseSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)

	cli = CLIOptions{Image: "alpine", ImageSet: true, Memory: "invalid", MemorySet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)

	cli = CLIOptions{Image: "alpine", ImageSet: true, PullMaxRetries: 0, PullMaxRetriesSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveWithFS_ExpressionError(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	cli := CLIOptions{Image: "{{file:missing}}", ImageSet: true}
	_, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)

	// Expression error in SocketPath
	cli = CLIOptions{Image: "alpine", ImageSet: true, SocketPath: "{{file:missing}}", SocketPathSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)

	// Expression error in Memory
	cli = CLIOptions{Image: "alpine", ImageSet: true, Memory: "{{file:missing}}", MemorySet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
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

func TestUnit_Coverage_Resolver_ResolveConfigPath_ExpressionError(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolver(nil)
	_, err := resolveConfigPath(true, "{{file:missing}}", false, "", "", "", nil, nil, nil, nil, "", r, "path", mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveConfigPath_Hierarchy(t *testing.T) {
	mfs := &MockFileSystem{WD: "/app"}
	r, _ := NewExpressionResolver(nil)

	// Tool config match
	tools := ToolsConfig{"node": {MountCderunPath: ConfigPath{Raw: "/tool/path"}}}
	res, err := resolveConfigPath(false, "", false, "", "", "node", tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath }, nil, nil, "/fallback", r, "path", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/tool/path", res)

	// Tool config exists but empty, falls back to global
	tools = ToolsConfig{"node": {}}
	global := &CDERunConfig{Defaults: ConfigDefaults{MountCderunPath: ConfigPath{Raw: "/global/path"}}}
	res, err = resolveConfigPath(false, "", false, "", "", "node", tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath }, global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath }, "/fallback", r, "path", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/global/path", res)

	// Both tool and global empty, falls back to fallback
	global = &CDERunConfig{}
	res, err = resolveConfigPath(false, "", false, "", "", "node", tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath }, global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath }, "/fallback", r, "path", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/fallback", res)

	// Test "volume" and "device" path types
	res, err = resolveConfigPath(true, "vol", false, "", "", "", nil, nil, nil, nil, "", r, "volume", mfs)
	require.NoError(t, err)
	assert.Equal(t, "vol", res)

	res, err = resolveConfigPath(true, "dev", false, "", "", "", nil, nil, nil, nil, "", r, "device", mfs)
	require.NoError(t, err)
	expected := filepath.Join(r.Pwd, "dev")
	assert.Equal(t, expected, res)
}

func TestUnit_Coverage_Resolver_ResolveDevices_Env(t *testing.T) {
	mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DEVICE": "/dev/a, /dev/b:/dev/c, "}}
	r, _ := NewExpressionResolver(nil)
	res, err := resolveDevices(nil, nil, "sh", nil, nil, r, mfs)
	require.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, "/dev/a", res[0].PathOnHost)
	assert.Equal(t, "/dev/b", res[1].PathOnHost)

	mfs.Env["CDERUN_DEVICE"] = ":" // Invalid: empty host and container
	_, err = resolveDevices(nil, nil, "sh", nil, nil, r, mfs)
	require.Error(t, err)
}

func TestUnit_Coverage_Resolver_ResolveEnv_Strict(t *testing.T) {
	mfs := &MockFileSystem{Env: map[string]string{"CDERUN_ENV": "A=1;B;C=3"}}
	r, _ := NewExpressionResolver(nil)

	// B is missing from host environment
	_, err := resolveEnv(nil, nil, "CDERUN_ENV", "sh", nil, nil, true, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required environment variable not found: \"B\"")

	// B exists on host
	mfs.Env["B"] = "2"
	res, err := resolveEnv(nil, nil, "CDERUN_ENV", "sh", nil, nil, true, r, mfs)
	require.NoError(t, err)
	assert.Len(t, res, 3)
	assert.Contains(t, res, "B=2")
}

func TestUnit_Coverage_Resolver_ResolveMounts_Optional_Exists(t *testing.T) {
	mfs := &MockFileSystem{
		Files: map[string][]byte{"/exists": {}},
		WD:    "/",
	}
	mcs := []string{"source=/exists,target=/t,optional"}

	t.Run("success", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, mfs)
		res, err := resolveMounts(mcs, nil, "sh", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "/exists", res[0].Source)
	})

	t.Run("stat error", func(t *testing.T) {
		mfs.StatErr = errors.New("perm denied")
		defer func() { mfs.StatErr = nil }()
		r, _ := NewExpressionResolverWithFS(nil, mfs)
		_, err := resolveMounts(mcs, nil, "sh", nil, nil, r, mfs)
		require.Error(t, err)
	})
}

func TestUnit_Coverage_Resolver_ResolveEnv_NonStrict_Found(t *testing.T) {
	mfs := &MockFileSystem{Env: map[string]string{"CDERUN_ENV": "B", "B": "2"}}
	r, _ := NewExpressionResolver(nil)
	res, err := resolveEnv(nil, nil, "CDERUN_ENV", "sh", nil, nil, false, r, mfs)
	require.NoError(t, err)
	assert.Contains(t, res, "B=2")
}

func TestUnit_Coverage_Path_ResolvePath_WithResolver(t *testing.T) {
	mfs := &MockFileSystem{HomeDir: "/home", WD: "/base"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Test ResolvePath with a minimal ExpressionResolver to verify home (~) expansion using MockFileSystem
	res, err := ResolvePath("~/foo", "/base", r)
	require.NoError(t, err)
	// On Linux mock filesystem it should use /home
	assert.Equal(t, "/home/foo", res)

	// Test with scheme
	res, err = ResolvePath("http://example.com", "/base", nil)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com", res)
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
	_, err := NewExpressionResolverWithFS(nil, mfs)
	require.Error(t, err)

	mfs = &CoverageErrorFS{MockFileSystem: &MockFileSystem{}}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

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

	// Verify independence
	cp.Devices[0].Source.Raw = "/mutated-d"
	cp.Mounts[0].Source.Raw = "/mutated-s"
	cp.Mounts[0].Target.Raw = "/mutated-t"

	assert.Equal(t, "/mutated-d", cp.Devices[0].Source.Raw)
	assert.Equal(t, "/mutated-s", cp.Mounts[0].Source.Raw)
	assert.Equal(t, "/mutated-t", cp.Mounts[0].Target.Raw)

	assert.Equal(t, "/d", d.Devices[0].Source.Raw)
	assert.Equal(t, "/s", d.Mounts[0].Source.Raw)
	assert.Equal(t, "/t", d.Mounts[0].Target.Raw)
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
	assert.InDelta(t, 3.14, resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{}), 1e-9)
	def.Fallback = nil
	assert.InDelta(t, 0.0, resolveFloat64Opt(def, false, 0, false, 0, "s", nil, nil, &MockFileSystem{}), 1e-9)
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
	require.NoError(t, err)
	assert.Nil(t, cfg)
	assert.Nil(t, paths)

	tcfg, tpaths, err := l.LoadToolsConfig()
	require.NoError(t, err)
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
	origRun := defaultLoader.runConfigDir
	origSys := defaultLoader.systemConfigDir

	r1 := SetRunConfigDirForTest("/tmp/run")
	t.Cleanup(r1)
	r2 := SetSystemConfigDirForTest("/tmp/sys")
	t.Cleanup(r2)

	assert.Equal(t, "/tmp/run", defaultLoader.runConfigDir)
	assert.Equal(t, "/tmp/sys", defaultLoader.systemConfigDir)

	r1()
	r2()

	assert.Equal(t, origRun, defaultLoader.runConfigDir)
	assert.Equal(t, origSys, defaultLoader.systemConfigDir)
}

func TestUnit_Coverage_Resolver_ResolveWithFS_ValidationExhaustive(t *testing.T) {
	mfs := &MockFileSystem{}

	// No image mapping
	_, err := ResolveWithFS("missing", &CLIOptions{}, nil, nil, mfs)
	var imgErr *ImageNotFoundError
	require.ErrorAs(t, err, &imgErr)
	assert.Equal(t, "missing", imgErr.Tool)

	// Negative hang-timeout
	cli := CLIOptions{Image: "a", ImageSet: true, HangTimeout: "-1s", HangTimeoutSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	var cfgErr *InvalidConfigError
	require.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, "hang-timeout", cfgErr.Field)
	assert.Contains(t, cfgErr.Error(), "duration cannot be negative")

	// Non-positive pull-max-retries
	cli = CLIOptions{Image: "a", ImageSet: true, PullMaxRetries: 0, PullMaxRetriesSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, "pull-max-retries", cfgErr.Field)
	assert.Contains(t, cfgErr.Error(), "must be greater than 0")

	// Non-positive pull-backoff-base
	cli = CLIOptions{Image: "a", ImageSet: true, PullBackoffBase: "0s", PullBackoffBaseSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, "pull-backoff-base", cfgErr.Field)
	assert.Contains(t, cfgErr.Error(), "must be positive")

	// Invalid pull-backoff-base format
	cli = CLIOptions{Image: "a", ImageSet: true, PullBackoffBase: "invalid", PullBackoffBaseSet: true}
	_, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, "pull-backoff-base", cfgErr.Field)
}

func TestUnit_Coverage_Resolver_ResolveWithFS_TransitiveLogic(t *testing.T) {
	mfs := &MockFileSystem{}

	// Case 1: mount-tools triggers mount-cderun and mount-socket
	cli := CLIOptions{Image: "a", ImageSet: true, MountTools: "node", MountToolsSet: true}
	res, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.True(t, res.MountCderun)
	assert.True(t, res.MountSocket)

	// Case 2: mount-all-tools triggers mount-cderun and mount-socket
	cli = CLIOptions{Image: "a", ImageSet: true, MountAllTools: true, MountAllToolsSet: true}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.True(t, res.MountCderun)
	assert.True(t, res.MountSocket)

	// Case 3: Explicit override breaks the chain
	cli = CLIOptions{
		Image: "a", ImageSet: true,
		MountTools: "node", MountToolsSet: true,
		MountCderun: false, MountCderunSet: true,
	}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.False(t, res.MountCderun)
	assert.False(t, res.MountSocket) // Also false because it defaults to MountCderun

	// Case 4: Explicit mount-socket override
	cli = CLIOptions{
		Image: "a", ImageSet: true,
		MountCderun: true, MountCderunSet: true,
		MountSocket: false, MountSocketSet: true,
	}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.True(t, res.MountCderun)
	assert.False(t, res.MountSocket)
}

func TestUnit_Coverage_Resolver_ResolveWithFS_SocketAutoDetection(t *testing.T) {
	// Case 1: Docker socket exists
	mfs := &MockFileSystem{
		Dirs: map[string]bool{"/var/run/docker.sock": true},
	}
	cli := CLIOptions{Image: "a", ImageSet: true}
	res, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "docker", res.Runtime)
	assert.Equal(t, "/var/run/docker.sock", res.SocketPath)
	assert.Equal(t, "/var/run/docker.sock", res.MountSocketPath) // Fallback to SocketPath

	// Case 2: Podman socket exists
	mfs = &MockFileSystem{
		Dirs: map[string]bool{"/run/podman/podman.sock": true},
	}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "podman", res.Runtime)
	assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

	// Case 3: Explicit Runtime Podman without SocketPath
	cli = CLIOptions{Image: "a", ImageSet: true, Runtime: "podman", RuntimeSet: true}
	mfs = &MockFileSystem{}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "podman", res.Runtime)
	assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

	// Case 4: Explicit SocketPath with podman in it
	cli = CLIOptions{Image: "a", ImageSet: true, SocketPath: "/tmp/podman.sock", SocketPathSet: true}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "podman", res.Runtime)
	assert.Equal(t, "/tmp/podman.sock", res.SocketPath)

	// Case 5: unix:// prefix removal
	cli = CLIOptions{Image: "a", ImageSet: true, SocketPath: "unix:///tmp/docker.sock", SocketPathSet: true}
	res, err = ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "/tmp/docker.sock", res.SocketPath)
}

func saveAndReplaceBoolOptionsMap(t *testing.T, newMap map[string]BoolOption) {
	t.Helper()
	old := boolOptionsMap
	boolOptionsMap = newMap
	t.Cleanup(func() {
		boolOptionsMap = old
	})
}

func TestUnit_Coverage_Resolver_ResolveWithFS_RegistryMismatchErrors(t *testing.T) {
	// Registry mismatch: 'mount-all-tools' not found (simulated by corrupted registry)
	saveAndReplaceBoolOptionsMap(t, make(map[string]BoolOption))

	mfs := &MockFileSystem{}
	cli := CLIOptions{Image: "a", ImageSet: true}
	_, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch: early boolean option \"diagnosis\" not found")
}

func TestUnit_Coverage_Resolver_resolveConfigPath_Modes(t *testing.T) {
	mfs := &MockFileSystem{}
	r, _ := NewExpressionResolver(nil)

	// Mode: volume
	res, err := resolveConfigPath(true, "/v", false, "", "", "s", nil, nil, nil, nil, "", r, "volume", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/v", res)

	// Mode: device
	res, err = resolveConfigPath(true, "/d", false, "", "", "s", nil, nil, nil, nil, "", r, "device", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/d", res)

	// Fallback to ENV (should override tools and global)
	mfs.Env = map[string]string{"CDERUN_SOCKET_PATH": "/env/socket"}
	res, err = resolveConfigPath(false, "", false, "", "CDERUN_SOCKET_PATH", "node",
		ToolsConfig{"node": ToolConfig{MountCderunPath: ConfigPath{Raw: "/tools/path"}}},
		func(t ToolConfig) ConfigPath { return t.MountCderunPath },
		&CDERunConfig{Defaults: ConfigDefaults{MountCderunPath: ConfigPath{Raw: "/global/path"}}},
		func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath },
		"", r, "path", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/env/socket", res)
	mfs.Env = nil // Cleanup

	// Fallback to tools
	tools := ToolsConfig{"node": ToolConfig{MountCderunPath: ConfigPath{Raw: "/tools/cderun"}}}
	res, err = resolveConfigPath(false, "", false, "", "", "node", tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath }, nil, nil, "", r, "path", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/tools/cderun", res)

	// Fallback to global
	global := &CDERunConfig{Defaults: ConfigDefaults{MountCderunPath: ConfigPath{Raw: "/global/cderun"}}}
	res, err = resolveConfigPath(false, "", false, "", "", "node", nil, func(t ToolConfig) ConfigPath { return t.MountCderunPath }, global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath }, "", r, "path", mfs)
	require.NoError(t, err)
	assert.Equal(t, "/global/cderun", res)
}

func TestUnit_Coverage_Resolver_resolveDevices_Errors(t *testing.T) {
	mfs := &MockFileSystem{}
	r, _ := NewExpressionResolver(nil)

	// Malformed in p1 (empty container path)
	_, err := resolveDevices([]string{"a:"}, nil, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid device config (override)")

	// Malformed in p2 (empty host path)
	_, err = resolveDevices(nil, []string{":b"}, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid device config")

	// Malformed in Env (empty parts)
	mfs.Env = map[string]string{"CDERUN_DEVICE": "a:"}
	_, err = resolveDevices(nil, nil, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid device config in CDERUN_DEVICE")
}

func TestUnit_Coverage_Resolver_resolveMounts_Errors(t *testing.T) {
	mfs := &MockFileSystem{}
	r, _ := NewExpressionResolverWithFS(nil, mfs)

	// Malformed in p1
	_, err := resolveMounts([]string{"invalid"}, nil, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mount config (override)")

	// Malformed in p2
	_, err = resolveMounts(nil, []string{"invalid"}, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mount config")

	// Malformed in Env
	mfs.Env = map[string]string{"CDERUN_MOUNT": "invalid"}
	_, err = resolveMounts(nil, nil, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mount config in CDERUN_MOUNT")

	// Optional mount Stat error (other than NotExist)
	mfs.Env = map[string]string{"CDERUN_MOUNT": "source=/s,target=/t,optional"}
	mfs.StatErr = errors.New("perm denied")
	_, err = resolveMounts(nil, nil, "s", nil, nil, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perm denied")
}

func TestUnit_Coverage_Resolver_resolveEnv_Strict(t *testing.T) {
	mfs := &MockFileSystem{}
	r, _ := NewExpressionResolver(nil)

	// Strict env failure
	_, err := resolveEnv(nil, []string{"MISSING"}, "E", "s", nil, nil, true, r, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required environment variable not found")
}

func TestUnit_Coverage_Resolver_NumericTypeMismatch(t *testing.T) {
	// To cover the 'else' branch in extractIntValue/extractFloatValue where a field's Kind is not numeric,
	// we manually manipulate fieldInfo to point an Int/Float option to a String field in CLIOptions,
	// while keeping the target field in ResolvedConfig numeric to avoid panic on SetInt/SetFloat.

	fieldOnce.Do(initFieldInfo)

	t.Run("Int mismatch", func(t *testing.T) {
		info := fieldInfo["pull-max-retries"]
		origP1ValIdx := info.p1ValIdx
		origP2ValIdx := info.p2ValIdx

		// Point CLI value lookups to 'Image' (string) instead of 'PullMaxRetries' (int)
		imgField, _ := cliType.FieldByName("CderunImage")
		info.p1ValIdx = imgField.Index[0]
		imgField2, _ := cliType.FieldByName("Image")
		info.p2ValIdx = imgField2.Index[0]
		fieldInfo["pull-max-retries"] = info

		defer func() {
			info.p1ValIdx = origP1ValIdx
			info.p2ValIdx = origP2ValIdx
			fieldInfo["pull-max-retries"] = info
		}()

		mfs := &MockFileSystem{}
		// Set string values in fields that are now pointed to for pull-max-retries
		cli := CLIOptions{
			CderunImage: "not-an-int", CderunImageSet: true,
			Image: "also-not-int", ImageSet: true,
		}
		// ResolveWithFS will now encounter reflect.String when resolving pull-max-retries,
		// triggering the 'else' branch and setting p1Set/p2Set to false.
		res, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
		require.NoError(t, err)
		// It should fall back to default (3)
		assert.Equal(t, 3, res.PullMaxRetries)
	})

	t.Run("Float64 mismatch", func(t *testing.T) {
		info := fieldInfo["cpus"]
		origP1ValIdx := info.p1ValIdx

		imgField, _ := cliType.FieldByName("CderunImage")
		info.p1ValIdx = imgField.Index[0]
		fieldInfo["cpus"] = info

		defer func() {
			info.p1ValIdx = origP1ValIdx
			fieldInfo["cpus"] = info
		}()

		mfs := &MockFileSystem{}
		cli := CLIOptions{CderunImage: "not-a-float", CderunImageSet: true}
		res, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, res.CPUs, 1e-9)
	})
}

func TestUnit_Coverage_Resolver_ResolveWithFS_NilCLI(t *testing.T) {
	mfs := &MockFileSystem{}
	// Should not panic and should return error due to missing image
	res, err := ResolveWithFS("node", nil, nil, nil, mfs)
	var imgErr *ImageNotFoundError
	require.ErrorAs(t, err, &imgErr)
	assert.Equal(t, "node", imgErr.Tool)
	assert.Nil(t, res)

	// Should work if diagnosis is requested via env
	mfs.Env = map[string]string{"CDERUN_DIAGNOSIS": "true"}
	res, err = ResolveWithFS("node", nil, nil, nil, mfs)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Diagnosis)
}

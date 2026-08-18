package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validValues[T any] struct {
	P1 T
	P2 T
	P3 string // env string representation
	P4 T
	P5 T
}

// Reference: docs/testing/strategy.md (L1: Priority Matrix) & docs/features/command-line-options.md
// TestUnit_Config_Registry_PriorityMatrix verifies priority resolution (P1 > P2 > P3 > P4 > P5 > P6)
// systematically across all registered option types in registry.go.
func TestUnit_Config_Registry_PriorityMatrix(t *testing.T) {
	t.Parallel()

	ensureRegistryMaps()

	// Specific valid value generators for options with strict validation or special defaults
	stringValidVals := map[string]validValues[string]{
		"pid":              {P1: "host", P2: "host", P3: "host", P4: "host", P5: "host"},
		"runtime":          {P1: "docker", P2: "podman", P3: "containerd", P4: "docker", P5: "podman"},
		"shm-size":         {P1: "512m", P2: "1g", P3: "2g", P4: "256m", P5: "64m"},
		"workdir":          {P1: "/app/p1", P2: "/app/p2", P3: "/app/p3", P4: "/app/p4", P5: "/app/p5"},
		"pull":             {P1: "always", P2: "never", P3: "missing", P4: "always", P5: "never"},
		"dry-run-format":   {P1: "json", P2: "simple", P3: "yaml", P4: "json", P5: "simple"},
		"diagnosis-format": {P1: "json", P2: "simple", P3: "yaml", P4: "json", P5: "simple"},
		"log-level":        {P1: "debug", P2: "info", P3: "warn", P4: "error", P5: "trace"},
		"log-format":       {P1: "json", P2: "text", P3: "json", P4: "text", P5: "json"},
		"ipc":              {P1: "host", P2: "private", P3: "host", P4: "private", P5: "host"},
		"cgroupns":         {P1: "host", P2: "private", P3: "host", P4: "private", P5: "host"},
		"cpuset-cpus":      {P1: "0-1", P2: "0-2", P3: "0-3", P4: "0", P5: "1"},
		"cpuset-mems":      {P1: "0-1", P2: "0-2", P3: "0-3", P4: "0", P5: "1"},
		"restart":          {P1: "always", P2: "no", P3: "on-failure", P4: "unless-stopped", P5: "no"},
	}

	stringSliceValidVals := map[string]validValues[[]string]{
		"publish":  {P1: []string{"8081:80"}, P2: []string{"8082:80"}, P3: "8083:80", P4: []string{"8084:80"}, P5: []string{"8085:80"}},
		"expose":   {P1: []string{"8081"}, P2: []string{"8082"}, P3: "8083", P4: []string{"8084"}, P5: []string{"8085"}},
		"dns":      {P1: []string{"8.8.8.8"}, P2: []string{"1.1.1.1"}, P3: "9.9.9.9", P4: []string{"8.8.4.4"}, P5: []string{"1.0.0.1"}},
		"add-host": {P1: []string{"h1:127.0.0.1"}, P2: []string{"h2:127.0.0.1"}, P3: "h3:127.0.0.1", P4: []string{"h4:127.0.0.1"}, P5: []string{"h5:127.0.0.1"}},
		"cap-add":  {P1: []string{"SYS_ADMIN"}, P2: []string{"NET_ADMIN"}, P3: "SYS_PTRACE", P4: []string{"MKNOD"}, P5: []string{"SYS_TIME"}},
		"cap-drop": {P1: []string{"KILL"}, P2: []string{"CHOWN"}, P3: "FOWNER", P4: []string{"SETUID"}, P5: []string{"SETGID"}},
	}

	t.Run("StringOptions_PriorityMatrix", func(t *testing.T) {
		for _, opt := range StringOptions {
			if opt.SkipResolution || opt.Name == "image" {
				continue // Skip custom-resolved or mandatory options like image
			}
			t.Run(opt.Name, func(t *testing.T) {
				fieldName := opt.FieldName
				if fieldName == "" {
					fieldName = PascalCase(opt.Name)
				}

				p1Val := "p1-" + opt.Name
				p2Val := "p2-" + opt.Name
				p3Val := "p3-" + opt.Name
				p4Val := "p4-" + opt.Name
				p5Val := "p5-" + opt.Name

				if vv, ok := stringValidVals[opt.Name]; ok {
					p1Val, p2Val, p3Val, p4Val, p5Val = vv.P1, vv.P2, vv.P3, vv.P4, vv.P5
				}

				makeCLI := func() *CLIOptions {
					cli := &CLIOptions{Image: ptr("alpine")}
					if opt.Name == "restart" {
						cli.Remove = ptr(false)
					}
					return cli
				}

				// Special handling for pid which only allows "host" or ""
				if opt.Name == "pid" {
					cliP1 := makeCLI()
					setP1String(t, cliP1, fieldName, "host")
					res, err := ResolveWithFS("sh", cliP1, nil, nil, &MockFileSystem{})
					require.NoError(t, err)
					assert.Equal(t, "host", res.Pid, "P1 should set host pid")

					cliP2 := makeCLI()
					setP2String(t, cliP2, fieldName, "host")
					res, err = ResolveWithFS("sh", cliP2, nil, nil, &MockFileSystem{})
					require.NoError(t, err)
					assert.Equal(t, "host", res.Pid, "P2 should set host pid")

					mfsP3 := &MockFileSystem{Env: map[string]string{opt.EnvKey: "host"}}
					res, err = ResolveWithFS("sh", makeCLI(), nil, nil, mfsP3)
					require.NoError(t, err)
					assert.Equal(t, "host", res.Pid, "P3 should set host pid")

					toolsP4 := createToolsConfigWithString(t, fieldName, "host")
					res, err = ResolveWithFS("sh", makeCLI(), toolsP4, nil, &MockFileSystem{})
					require.NoError(t, err)
					assert.Equal(t, "host", res.Pid, "P4 should set host pid")

					globalP5 := createGlobalConfigWithString(t, opt.Name, fieldName, "host")
					res, err = ResolveWithFS("sh", makeCLI(), nil, globalP5, &MockFileSystem{})
					require.NoError(t, err)
					assert.Equal(t, "host", res.Pid, "P5 should set host pid")
					return
				}

				// Test P1 > P2 > P3 > P4 > P5
				cliP1 := makeCLI()
				setP1String(t, cliP1, fieldName, p1Val)
				setP2String(t, cliP1, fieldName, p2Val)

				mfs := &MockFileSystem{Env: map[string]string{opt.EnvKey: p3Val}}
				var tools ToolsConfig
				if opt.ToolGetter != nil {
					tools = createToolsConfigWithString(t, fieldName, p4Val)
				}
				global := createGlobalConfigWithString(t, opt.Name, fieldName, p5Val)

				// Test P1 wins over all
				res, err := ResolveWithFS("sh", cliP1, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p1Val, getStringResult(t, res, fieldName), "P1 should win for "+opt.Name)

				// Test P2 wins over P3, P4, P5
				cliP2 := makeCLI()
				setP2String(t, cliP2, fieldName, p2Val)
				res, err = ResolveWithFS("sh", cliP2, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p2Val, getStringResult(t, res, fieldName), "P2 should win for "+opt.Name)

				// Test P3 wins over P4, P5
				cliEmpty := makeCLI()
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p3Val, getStringResult(t, res, fieldName), "P3 should win for "+opt.Name)

				// Test P4 wins over P5 (if ToolGetter is supported)
				if opt.ToolGetter != nil {
					mfsNoEnv := &MockFileSystem{}
					res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfsNoEnv)
					require.NoError(t, err)
					assert.Equal(t, p4Val, getStringResult(t, res, fieldName), "P4 should win for "+opt.Name)
				}

				// Test P5 wins over default
				if opt.Name == "runtime" {
					// runtime has auto-detection when P1-P5 are empty; verify P5 overrides auto-detected default
					globalP5 := &CDERunConfig{Runtime: p5Val}
					res, err := ResolveWithFS("sh", cliEmpty, nil, globalP5, &MockFileSystem{})
					require.NoError(t, err)
					assert.Equal(t, p5Val, res.Runtime, "P5 should win for runtime")
				} else {
					mfsNoEnv := &MockFileSystem{}
					res, err = ResolveWithFS("sh", cliEmpty, nil, global, mfsNoEnv)
					require.NoError(t, err)
					assert.Equal(t, p5Val, getStringResult(t, res, fieldName), "P5 should win for "+opt.Name)
				}
			})
		}
	})

	t.Run("BoolOptions_PriorityMatrix", func(t *testing.T) {
		for _, opt := range BoolOptions {
			if opt.Name == "diagnosis" || opt.Name == "strict-env" || opt.Name == "mount-socket" || opt.Name == "mount-cderun" || opt.Name == "mount-all-tools" {
				continue // Skip early or transitive options tested separately
			}
			t.Run(opt.Name, func(t *testing.T) {
				fieldName := opt.FieldName
				if fieldName == "" {
					fieldName = PascalCase(opt.Name)
				}

				winVal := !opt.Default
				loseVal := opt.Default

				// P1 vs P2
				cliP1 := &CLIOptions{Image: ptr("alpine")}
				setP1Bool(t, cliP1, fieldName, winVal)
				setP2Bool(t, cliP1, fieldName, loseVal)

				mfs := &MockFileSystem{Env: map[string]string{opt.EnvKey: boolToString(loseVal)}}
				var tools ToolsConfig
				if opt.ToolGetter != nil {
					tools = createToolsConfigWithBool(t, fieldName, loseVal)
				}
				global := createGlobalConfigWithBool(t, opt.Name, fieldName, loseVal)

				res, err := ResolveWithFS("sh", cliP1, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, winVal, getBoolResult(t, res, fieldName), "P1 should win for "+opt.Name)

				// P2 vs P3
				cliP2 := &CLIOptions{Image: ptr("alpine")}
				setP2Bool(t, cliP2, fieldName, winVal)
				mfsP3 := &MockFileSystem{Env: map[string]string{opt.EnvKey: boolToString(loseVal)}}
				res, err = ResolveWithFS("sh", cliP2, tools, global, mfsP3)
				require.NoError(t, err)
				assert.Equal(t, winVal, getBoolResult(t, res, fieldName), "P2 should win for "+opt.Name)

				// P3 vs P4
				cliEmpty := &CLIOptions{Image: ptr("alpine")}
				mfsP3Win := &MockFileSystem{Env: map[string]string{opt.EnvKey: boolToString(winVal)}}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfsP3Win)
				require.NoError(t, err)
				assert.Equal(t, winVal, getBoolResult(t, res, fieldName), "P3 should win for "+opt.Name)

				// P4 vs P5
				mfsEmpty := &MockFileSystem{}
				var toolsWin ToolsConfig
				if opt.ToolGetter != nil {
					toolsWin = createToolsConfigWithBool(t, fieldName, winVal)
				}
				globalLose := createGlobalConfigWithBool(t, opt.Name, fieldName, loseVal)
				res, err = ResolveWithFS("sh", cliEmpty, toolsWin, globalLose, mfsEmpty)
				require.NoError(t, err)
				assert.Equal(t, winVal, getBoolResult(t, res, fieldName), "P4 should win for "+opt.Name)

				// P5 vs Default
				globalWin := createGlobalConfigWithBool(t, opt.Name, fieldName, winVal)
				res, err = ResolveWithFS("sh", cliEmpty, nil, globalWin, mfsEmpty)
				require.NoError(t, err)
				assert.Equal(t, winVal, getBoolResult(t, res, fieldName), "P5 should win for "+opt.Name)
			})
		}
	})

	t.Run("IntOptions_PriorityMatrix", func(t *testing.T) {
		for _, opt := range IntOptions {
			t.Run(opt.Name, func(t *testing.T) {
				fieldName := opt.FieldName
				if fieldName == "" {
					fieldName = PascalCase(opt.Name)
				}

				p1Val, p2Val, p3Val, p4Val, p5Val := 10, 20, 30, 40, 50

				// P1 vs P2
				cliP1 := &CLIOptions{Image: ptr("alpine")}
				setP1Int(t, cliP1, fieldName, p1Val)
				setP2Int(t, cliP1, fieldName, p2Val)

				mfs := &MockFileSystem{Env: map[string]string{opt.EnvKey: "30"}}
				var tools ToolsConfig
				if opt.ToolGetter != nil {
					tools = createToolsConfigWithInt(t, fieldName, p4Val)
				}
				global := createGlobalConfigWithInt(t, fieldName, p5Val)

				res, err := ResolveWithFS("sh", cliP1, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p1Val, getIntResult(t, res, fieldName), "P1 should win for "+opt.Name)

				// P2 vs P3
				cliP2 := &CLIOptions{Image: ptr("alpine")}
				setP2Int(t, cliP2, fieldName, p2Val)
				res, err = ResolveWithFS("sh", cliP2, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p2Val, getIntResult(t, res, fieldName), "P2 should win for "+opt.Name)

				// P3 vs P4
				cliEmpty := &CLIOptions{Image: ptr("alpine")}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p3Val, getIntResult(t, res, fieldName), "P3 should win for "+opt.Name)

				// P4 vs P5
				mfsEmpty := &MockFileSystem{}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfsEmpty)
				require.NoError(t, err)
				assert.Equal(t, p4Val, getIntResult(t, res, fieldName), "P4 should win for "+opt.Name)

				// P5 vs Default
				res, err = ResolveWithFS("sh", cliEmpty, nil, global, mfsEmpty)
				require.NoError(t, err)
				assert.Equal(t, p5Val, getIntResult(t, res, fieldName), "P5 should win for "+opt.Name)
			})
		}
	})

	t.Run("Float64Options_PriorityMatrix", func(t *testing.T) {
		for _, opt := range Float64Options {
			t.Run(opt.Name, func(t *testing.T) {
				fieldName := opt.FieldName
				if fieldName == "" {
					fieldName = PascalCase(opt.Name)
				}

				p1Val, p2Val, p3Val, p4Val, p5Val := 1.5, 2.5, 3.5, 4.5, 5.5

				// P1 vs P2
				cliP1 := &CLIOptions{Image: ptr("alpine")}
				setP1Float64(t, cliP1, fieldName, p1Val)
				setP2Float64(t, cliP1, fieldName, p2Val)

				mfs := &MockFileSystem{Env: map[string]string{opt.EnvKey: "3.5"}}
				var tools ToolsConfig
				if opt.ToolGetter != nil {
					tools = createToolsConfigWithFloat64(t, fieldName, p4Val)
				}
				global := createGlobalConfigWithFloat64(t, fieldName, p5Val)

				res, err := ResolveWithFS("sh", cliP1, tools, global, mfs)
				require.NoError(t, err)
				assert.InDelta(t, p1Val, getFloat64Result(t, res, fieldName), 1e-9, "P1 should win for "+opt.Name)

				// P2 vs P3
				cliP2 := &CLIOptions{Image: ptr("alpine")}
				setP2Float64(t, cliP2, fieldName, p2Val)
				res, err = ResolveWithFS("sh", cliP2, tools, global, mfs)
				require.NoError(t, err)
				assert.InDelta(t, p2Val, getFloat64Result(t, res, fieldName), 1e-9, "P2 should win for "+opt.Name)

				// P3 vs P4
				cliEmpty := &CLIOptions{Image: ptr("alpine")}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfs)
				require.NoError(t, err)
				assert.InDelta(t, p3Val, getFloat64Result(t, res, fieldName), 1e-9, "P3 should win for "+opt.Name)

				// P4 vs P5
				mfsEmpty := &MockFileSystem{}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfsEmpty)
				require.NoError(t, err)
				assert.InDelta(t, p4Val, getFloat64Result(t, res, fieldName), 1e-9, "P4 should win for "+opt.Name)

				// P5 vs Default
				res, err = ResolveWithFS("sh", cliEmpty, nil, global, mfsEmpty)
				require.NoError(t, err)
				assert.InDelta(t, p5Val, getFloat64Result(t, res, fieldName), 1e-9, "P5 should win for "+opt.Name)
			})
		}
	})

	t.Run("StringSliceOptions_PriorityMatrix", func(t *testing.T) {
		for _, opt := range StringSliceOptions {
			if opt.SkipResolution || opt.Name == "env" || opt.Name == "mount" || opt.Name == "ulimit" || opt.Name == "device" || opt.Name == "sensitive-env" {
				continue // Skip custom-resolved slices
			}
			t.Run(opt.Name, func(t *testing.T) {
				fieldName := opt.FieldName
				if fieldName == "" {
					fieldName = PascalCase(opt.Name)
				}

				p1Val := []string{"p1a", "p1b"}
				p2Val := []string{"p2a", "p2b"}
				p3Val := "p3a,p3b"
				p3Expected := []string{"p3a", "p3b"}
				p4Val := []string{"p4a", "p4b"}
				p5Val := []string{"p5a", "p5b"}

				if vv, ok := stringSliceValidVals[opt.Name]; ok {
					p1Val, p2Val, p3Val, p4Val, p5Val = vv.P1, vv.P2, vv.P3, vv.P4, vv.P5
					p3Expected = []string{vv.P3}
				}

				// P1 vs P2
				cliP1 := &CLIOptions{Image: ptr("alpine")}
				setP1Slice(t, cliP1, fieldName, p1Val)
				setP2Slice(t, cliP1, fieldName, p2Val)

				mfs := &MockFileSystem{Env: map[string]string{opt.EnvKey: p3Val}}
				var tools ToolsConfig
				if opt.ToolGetter != nil {
					tools = createToolsConfigWithSlice(t, fieldName, p4Val)
				}
				global := createGlobalConfigWithSlice(t, fieldName, p5Val)

				res, err := ResolveWithFS("sh", cliP1, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p1Val, getSliceResult(t, res, fieldName), "P1 should win for "+opt.Name)

				// P2 vs P3
				cliP2 := &CLIOptions{Image: ptr("alpine")}
				setP2Slice(t, cliP2, fieldName, p2Val)
				res, err = ResolveWithFS("sh", cliP2, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p2Val, getSliceResult(t, res, fieldName), "P2 should win for "+opt.Name)

				// P3 vs P4
				cliEmpty := &CLIOptions{Image: ptr("alpine")}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfs)
				require.NoError(t, err)
				assert.Equal(t, p3Expected, getSliceResult(t, res, fieldName), "P3 should win for "+opt.Name)

				// P4 vs P5
				mfsEmpty := &MockFileSystem{}
				res, err = ResolveWithFS("sh", cliEmpty, tools, global, mfsEmpty)
				require.NoError(t, err)
				assert.Equal(t, p4Val, getSliceResult(t, res, fieldName), "P4 should win for "+opt.Name)

				// P5 vs Default
				res, err = ResolveWithFS("sh", cliEmpty, nil, global, mfsEmpty)
				require.NoError(t, err)
				assert.Equal(t, p5Val, getSliceResult(t, res, fieldName), "P5 should win for "+opt.Name)
			})
		}
	})
}

// Reflection helpers for matrix test execution

func setP1String(t *testing.T, cli *CLIOptions, fieldName string, val string) {
	v := reflect.ValueOf(cli).Elem().FieldByName("Cderun" + fieldName)
	require.True(t, v.IsValid(), "field Cderun"+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field Cderun"+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func setP2String(t *testing.T, cli *CLIOptions, fieldName string, val string) {
	v := reflect.ValueOf(cli).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func getStringResult(t *testing.T, res *ResolvedConfig, fieldName string) string {
	v := reflect.ValueOf(res).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "result field "+fieldName+" must be valid")
	return v.String()
}

func setP1Bool(t *testing.T, cli *CLIOptions, fieldName string, val bool) {
	v := reflect.ValueOf(cli).Elem().FieldByName("Cderun" + fieldName)
	require.True(t, v.IsValid(), "field Cderun"+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field Cderun"+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func setP2Bool(t *testing.T, cli *CLIOptions, fieldName string, val bool) {
	v := reflect.ValueOf(cli).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func getBoolResult(t *testing.T, res *ResolvedConfig, fieldName string) bool {
	v := reflect.ValueOf(res).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "result field "+fieldName+" must be valid")
	return v.Bool()
}

func setP1Int(t *testing.T, cli *CLIOptions, fieldName string, val int) {
	v := reflect.ValueOf(cli).Elem().FieldByName("Cderun" + fieldName)
	require.True(t, v.IsValid(), "field Cderun"+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field Cderun"+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func setP2Int(t *testing.T, cli *CLIOptions, fieldName string, val int) {
	v := reflect.ValueOf(cli).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func getIntResult(t *testing.T, res *ResolvedConfig, fieldName string) int {
	v := reflect.ValueOf(res).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "result field "+fieldName+" must be valid")
	return int(v.Int())
}

func setP1Float64(t *testing.T, cli *CLIOptions, fieldName string, val float64) {
	v := reflect.ValueOf(cli).Elem().FieldByName("Cderun" + fieldName)
	require.True(t, v.IsValid(), "field Cderun"+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field Cderun"+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func setP2Float64(t *testing.T, cli *CLIOptions, fieldName string, val float64) {
	v := reflect.ValueOf(cli).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
}

func getFloat64Result(t *testing.T, res *ResolvedConfig, fieldName string) float64 {
	v := reflect.ValueOf(res).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "result field "+fieldName+" must be valid")
	return v.Float()
}

func setP1Slice(t *testing.T, cli *CLIOptions, fieldName string, val []string) {
	v := reflect.ValueOf(cli).Elem().FieldByName("Cderun" + fieldName)
	require.True(t, v.IsValid(), "field Cderun"+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field Cderun"+fieldName+" must be settable")
	v.Set(reflect.ValueOf(val))
}

func setP2Slice(t *testing.T, cli *CLIOptions, fieldName string, val []string) {
	v := reflect.ValueOf(cli).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(val))
}

func getSliceResult(t *testing.T, res *ResolvedConfig, fieldName string) []string {
	v := reflect.ValueOf(res).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "result field "+fieldName+" must be valid")
	require.False(t, v.IsNil(), "result field "+fieldName+" must not be nil")
	return v.Interface().([]string)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func createToolsConfigWithString(t *testing.T, fieldName string, val string) ToolsConfig {
	tc := ToolConfig{}
	v := reflect.ValueOf(&tc).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ToolConfig field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ToolConfig field "+fieldName+" must be settable")
	v.SetString(val)
	return ToolsConfig{"sh": tc}
}

func createGlobalConfigWithString(t *testing.T, optName, fieldName string, val string) *CDERunConfig {
	cfg := &CDERunConfig{}
	if optName == "log-level" || optName == "log-format" {
		if optName == "log-level" {
			cfg.Logging.Level = val
		} else {
			cfg.Logging.Format = val
		}
		return cfg
	}
	if optName == "runtime" {
		cfg.Runtime = val
		return cfg
	}
	v := reflect.ValueOf(&cfg.Defaults).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ConfigDefaults field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ConfigDefaults field "+fieldName+" must be settable")
	v.SetString(val)
	return cfg
}

func createToolsConfigWithBool(t *testing.T, fieldName string, val bool) ToolsConfig {
	tc := ToolConfig{}
	v := reflect.ValueOf(&tc).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ToolConfig field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ToolConfig field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
	return ToolsConfig{"sh": tc}
}

func createGlobalConfigWithBool(t *testing.T, optName, fieldName string, val bool) *CDERunConfig {
	cfg := &CDERunConfig{}
	if optName == "log-timestamp" {
		cfg.Logging.Timestamp = &val
		return cfg
	}
	v := reflect.ValueOf(&cfg.Defaults).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ConfigDefaults field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ConfigDefaults field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
	return cfg
}

func createToolsConfigWithInt(t *testing.T, fieldName string, val int) ToolsConfig {
	tc := ToolConfig{}
	v := reflect.ValueOf(&tc).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ToolConfig field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ToolConfig field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
	return ToolsConfig{"sh": tc}
}

func createGlobalConfigWithInt(t *testing.T, fieldName string, val int) *CDERunConfig {
	cfg := &CDERunConfig{}
	v := reflect.ValueOf(&cfg.Defaults).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ConfigDefaults field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ConfigDefaults field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
	return cfg
}

func createToolsConfigWithFloat64(t *testing.T, fieldName string, val float64) ToolsConfig {
	tc := ToolConfig{}
	v := reflect.ValueOf(&tc).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ToolConfig field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ToolConfig field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
	return ToolsConfig{"sh": tc}
}

func createGlobalConfigWithFloat64(t *testing.T, fieldName string, val float64) *CDERunConfig {
	cfg := &CDERunConfig{}
	v := reflect.ValueOf(&cfg.Defaults).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ConfigDefaults field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ConfigDefaults field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(&val))
	return cfg
}

func createToolsConfigWithSlice(t *testing.T, fieldName string, val []string) ToolsConfig {
	tc := ToolConfig{}
	v := reflect.ValueOf(&tc).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ToolConfig field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ToolConfig field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(val))
	return ToolsConfig{"sh": tc}
}

func createGlobalConfigWithSlice(t *testing.T, fieldName string, val []string) *CDERunConfig {
	cfg := &CDERunConfig{}
	v := reflect.ValueOf(&cfg.Defaults).Elem().FieldByName(fieldName)
	require.True(t, v.IsValid(), "ConfigDefaults field "+fieldName+" must be valid")
	require.True(t, v.CanSet(), "ConfigDefaults field "+fieldName+" must be settable")
	v.Set(reflect.ValueOf(val))
	return cfg
}

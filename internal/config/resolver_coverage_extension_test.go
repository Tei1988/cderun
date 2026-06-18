package config

import (
	"cderun/internal/logging"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withPatchedFieldInfo mutates fieldInfo for testing and restores it after the test.
func withPatchedFieldInfo(t *testing.T, key string, mutation func()) {
	t.Helper()
	fieldOnce.Do(initFieldInfo)
	orig, exists := fieldInfo[key]
	t.Cleanup(func() {
		if exists {
			fieldInfo[key] = orig
		} else {
			delete(fieldInfo, key)
		}
	})
	mutation()
}

func TestUnit_Config_FieldInfo_ErrorPaths(t *testing.T) {
	cliVal := reflect.ValueOf(&CLIOptions{}).Elem()

	t.Run("missing registry info", func(t *testing.T) {
		_, _, _, _, _, err := fetchFieldAndParams("nonexistent", cliVal)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"nonexistent\" not found")
	})

	t.Run("missing reflection fields", func(t *testing.T) {
		withPatchedFieldInfo(t, "image", func() {
			info := fieldInfo["image"]
			info.p1ValIdx = -1
			fieldInfo["image"] = info
		})
		_, _, _, _, _, err := fetchFieldAndParams("image", cliVal)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: CLI reflection fields for option \"image\" missing")
	})
}

func TestUnit_Config_GetFieldInfo_Exhaustive(t *testing.T) {
	type testStruct struct {
		Ptr       *string
		Interface any
		Map       map[string]string
		Zero      int
	}
	s := "test"
	ts := testStruct{
		Ptr:       &s,
		Interface: s,
		Map:       map[string]string{"a": "b"},
		Zero:      0,
	}
	val := reflect.ValueOf(ts)

	t.Run("Ptr set", func(t *testing.T) {
		set, _ := getFieldInfo(val, -1, 0)
		assert.True(t, set)
	})

	t.Run("Interface set", func(t *testing.T) {
		set, _ := getFieldInfo(val, -1, 1)
		assert.True(t, set)
	})

	t.Run("Map set", func(t *testing.T) {
		set, _ := getFieldInfo(val, -1, 2)
		assert.True(t, set)
	})

	t.Run("Zero not set", func(t *testing.T) {
		set, _ := getFieldInfo(val, -1, 3)
		assert.False(t, set)
	})

	tsNil := testStruct{
		Ptr:       nil,
		Interface: nil,
		Map:       nil,
	}
	valNil := reflect.ValueOf(tsNil)

	t.Run("Ptr nil", func(t *testing.T) {
		set, _ := getFieldInfo(valNil, -1, 0)
		assert.False(t, set)
	})

	t.Run("Interface nil", func(t *testing.T) {
		set, _ := getFieldInfo(valNil, -1, 1)
		assert.False(t, set)
	})

	t.Run("Map nil", func(t *testing.T) {
		set, _ := getFieldInfo(valNil, -1, 2)
		assert.False(t, set)
	})
}

func TestUnit_Config_ResolveWithFS_Coverage(t *testing.T) {
	// Not Parallel because it mutates global fieldInfo or global logger level

	t.Run("registry mismatch for early boolean option", func(t *testing.T) {
		withPatchedFieldInfo(t, "diagnosis", func() {
			delete(fieldInfo, "diagnosis")
		})

		_, err := ResolveWithFS("", nil, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"diagnosis\" not found")
	})

	t.Run("registry mismatch for string option in phase 2", func(t *testing.T) {
		withPatchedFieldInfo(t, "image", func() {
			delete(fieldInfo, "image")
		})

		_, err := ResolveWithFS("", nil, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"image\" not found")
	})

	t.Run("registry mismatch validation with expression error", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_IMAGE": "my-reg.com/node:{{env:UNKNOWN}}"},
		}
		tools := ToolsConfig{
			// The registry check logic uses ResolveString which sets the sticky error if it fails.
			// However, ResolveWithFS calls validateImageRegistryMatch which uses r.ResolveString.
			// If r.ResolveString fails, it returns the error, BUT validateImageRegistryMatch
			// only fails if both errCLI and errCfg are nil.
			"node": ToolConfig{Image: "my-reg.com/node:18"},
		}
		res, err := ResolveWithFS("node", nil, tools, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Image, "my-reg.com/node:")
	})

	t.Run("registry mismatch validation with config expression error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		tools := ToolsConfig{
			"node": ToolConfig{Image: "my-reg.com/node:{{env:UNKNOWN}}"},
		}
		cli := &CLIOptions{Image: "my-reg.com/node:20", ImageSet: true}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "my-reg.com/node:20", res.Image)
	})

	t.Run("negative duration in tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{Image: "alpine", HangTimeout: "-1s"},
		}
		_, err := ResolveWithFS("node", nil, tools, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duration cannot be negative")
	})

	t.Run("invalid duration in tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{Image: "alpine", HangTimeout: "invalid"},
		}
		_, err := ResolveWithFS("node", nil, tools, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hang-timeout value")
	})

	t.Run("pull-backoff-base invalid in tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{Image: "alpine", PullBackoffBase: "invalid"},
		}
		_, err := ResolveWithFS("node", nil, tools, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull-backoff-base value")
	})

	t.Run("pull-backoff-base zero in tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{Image: "alpine", PullBackoffBase: "0s"},
		}
		_, err := ResolveWithFS("node", nil, tools, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})

	t.Run("memory invalid in tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{Image: "alpine", Memory: "invalid"},
		}
		_, err := ResolveWithFS("node", nil, tools, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid memory value")
	})

	t.Run("security validation failure for user", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, User: "user\nname", UserSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for \"user\"")
	})

	t.Run("security validation failure for env key", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"BAD\nKEY=val"}}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid environment variable key")
	})

	t.Run("security validation failure for mounts", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Mounts:   []string{"source=/bad\n,target=/good"},
		}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for mounts[0] (source)")

		cli = &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Mounts:   []string{"source=/good,target=/bad\r"},
		}
		_, err = ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for mounts[0] (target)")
	})

	t.Run("security validation failure for devices", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Devices:  []string{"/bad\n:/good"},
		}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for devices[0] (path-on-host)")

		cli = &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Devices:  []string{"/good:/bad\r"},
		}
		_, err = ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for devices[0] (path-in-container)")
	})

	t.Run("resolveDevices fallback to global", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Devices: []DeviceConfig{{Source: ConfigPath{Raw: "/dev/global"}, Destination: ConfigPath{Raw: "/dev/global"}}},
			},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/global", res.Devices[0].PathOnHost)
	})

	t.Run("resolveEnv fallback to global", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Env: []string{"GLOBAL=1"},
			},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Contains(t, res.Env, "GLOBAL=1")
	})

	t.Run("resolveDevices with empty segments in environment", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_DEVICE": "/dev/a:/dev/a , , /dev/b:/dev/b"},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Devices, 2)
	})

	t.Run("resolveEnv with empty segments in environment", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_ENV": "A=1 ; ; B=2"},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "A=1")
		assert.Contains(t, res.Env, "B=2")
		assert.Len(t, res.Env, 2)
	})

	t.Run("resolveMounts fallback to global", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Mounts: []MountConfig{{Source: ConfigPath{Raw: "/src"}, Target: ConfigPath{Raw: "/dst"}}},
			},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/src", res.Mounts[0].Source)
	})

	t.Run("transitive options: mount-all-tools", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, MountAllTools: true, MountAllToolsSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.True(t, res.MountAllTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("transitive options: mount-tools", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, MountTools: "git", MountToolsSet: true}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, []string{"git"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("RAMInBytes error with sticky expression error", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Env:       []string{"FOO={{file:missing}}"},
			Memory:    "invalid",
			MemorySet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})


	t.Run("security validation failure for various fields", func(t *testing.T) {
		fields := []struct {
			name string
			cli  CLIOptions
		}{
			{"network", CLIOptions{Network: "net\r", NetworkSet: true}},
			{"hostname", CLIOptions{Hostname: "host\t", HostnameSet: true}},
			{"workdir", CLIOptions{Workdir: "dir\v", WorkdirSet: true}},
			{"runtime", CLIOptions{Runtime: "run\f", RuntimeSet: true}},
			{"dry-run-format", CLIOptions{DryRunFormat: "fmt\n", DryRunFormatSet: true}},
			{"log-level", CLIOptions{LogLevel: "level\r", LogLevelSet: true}},
		}

		for _, f := range fields {
			t.Run(f.name, func(t *testing.T) {
				f.cli.Image = "alpine"
				f.cli.ImageSet = true
				_, err := ResolveWithFS("node", &f.cli, nil, nil, &MockFileSystem{})
				require.Error(t, err)
				assert.Contains(t, err.Error(), fmt.Sprintf("security validation failed for %q", f.name))
			})
		}
	})

	t.Run("security validation failure for slice elements", func(t *testing.T) {
		slices := []struct {
			name string
			cli  CLIOptions
		}{
			{"entrypoint", CLIOptions{Entrypoint: []string{"ep\n"}}},
			{"ports", CLIOptions{Ports: []string{"80\r"}}},
			{"expose", CLIOptions{Expose: []string{"80\t"}}},
			{"dns", CLIOptions{DNS: []string{"8.8.8.8\v"}}},
			{"add-hosts", CLIOptions{AddHosts: []string{"host:ip\f"}}},
			{"cap-add", CLIOptions{CapAdd: []string{"SYS_ADMIN\n"}}},
			{"cap-drop", CLIOptions{CapDrop: []string{"ALL\r"}}},
		}

		for _, s := range slices {
			t.Run(s.name, func(t *testing.T) {
				s.cli.Image = "alpine"
				s.cli.ImageSet = true
				_, err := ResolveWithFS("node", &s.cli, nil, nil, &MockFileSystem{})
				require.Error(t, err)
				assert.Contains(t, err.Error(), fmt.Sprintf("security validation failed for %s[0]", s.name))
			})
		}
	})

	t.Run("invalid tool name in mount-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:         "alpine",
			ImageSet:      true,
			MountTools:    "../bad",
			MountToolsSet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool name in mount-tools")
	})

	t.Run("pull-max-retries non-positive", func(t *testing.T) {
		cli := &CLIOptions{
			Image:             "alpine",
			ImageSet:          true,
			PullMaxRetries:    0,
			PullMaxRetriesSet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull-max-retries value \"0\"")
	})

	t.Run("trace logging", func(t *testing.T) {
		origLevel := logging.GetGlobalLogger().GetLevel()
		logging.GetGlobalLogger().SetLevel(logging.TraceLevel)
		defer logging.GetGlobalLogger().SetLevel(origLevel)

		cli := &CLIOptions{Image: "alpine", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
	})

	t.Run("security validation failure for image", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine\n", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for image")
	})

	t.Run("hang-timeout invalid duration", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, HangTimeout: "invalid", HangTimeoutSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hang-timeout value")
	})

	t.Run("pull-backoff-base invalid duration", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, CderunPullBackoffBase: "invalid", CderunPullBackoffBaseSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull-backoff-base value")
	})

	t.Run("pull-backoff-base non-positive", func(t *testing.T) {
		cli := &CLIOptions{Image: "alpine", ImageSet: true, CderunPullBackoffBase: "0s", CderunPullBackoffBaseSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})

	t.Run("transitive options: mount-cderun and mount-socket specified", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountCderun:    false,
			MountCderunSet: true,
			MountSocket:    false,
			MountSocketSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.False(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})

	t.Run("transitive options: mount-cderun overrides mount-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountTools:     "git",
			MountToolsSet:  true,
			MountCderun:    false,
			MountCderunSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.False(t, res.MountCderun)
		// socket is transitive from cderun, so it should also be false if not specified
		assert.False(t, res.MountSocket)
	})

	t.Run("transitive options: mount-socket false explicitly with mount-cderun true", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountCderun:    true,
			MountCderunSet: true,
			MountSocket:    false,
			MountSocketSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})

	t.Run("registry mismatch: only CLI image provided", func(t *testing.T) {
		cli := &CLIOptions{CderunImage: "my-reg.com/node:20", CderunImageSet: true}
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "my-reg.com/node:20", res.Image)
	})

	t.Run("memory expression error", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "{{file:missing}}",
			MemorySet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("applyIntOption with non-int field", func(t *testing.T) {
		orig := testIgnoreFastPath
		testIgnoreFastPath = true
		defer func() { testIgnoreFastPath = orig }()
		withPatchedFieldInfo(t, "pull-max-retries", func() {
			info := fieldInfo["pull-max-retries"]
			// Point p1ValIdx to a string field (Image)
			info.p1ValIdx = 0
			fieldInfo["pull-max-retries"] = info
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true, CderunPullMaxRetriesSet: true}
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		// Should fallback to default (3) because k is not int
		assert.Equal(t, 3, res.PullMaxRetries)
	})

	t.Run("applyFloat64Option with non-float field", func(t *testing.T) {
		orig := testIgnoreFastPath
		testIgnoreFastPath = true
		defer func() { testIgnoreFastPath = orig }()
		withPatchedFieldInfo(t, "cpus", func() {
			info := fieldInfo["cpus"]
			// Point p1ValIdx to a string field (Image)
			info.p1ValIdx = 0
			fieldInfo["cpus"] = info
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true, CderunCPUsSet: true}
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		// Should be 0.0 because k is not float
		assert.InDelta(t, 0.0, res.CPUs, 1e-9)
	})

	t.Run("applyIntOption p2 non-int", func(t *testing.T) {
		orig := testIgnoreFastPath
		testIgnoreFastPath = true
		defer func() { testIgnoreFastPath = orig }()
		withPatchedFieldInfo(t, "pull-max-retries", func() {
			info := fieldInfo["pull-max-retries"]
			info.p2ValIdx = 0
			fieldInfo["pull-max-retries"] = info
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true, PullMaxRetriesSet: true}
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, 3, res.PullMaxRetries)
	})

	t.Run("applyFloat64Option p2 non-float", func(t *testing.T) {
		orig := testIgnoreFastPath
		testIgnoreFastPath = true
		defer func() { testIgnoreFastPath = orig }()
		withPatchedFieldInfo(t, "cpus", func() {
			info := fieldInfo["cpus"]
			info.p2ValIdx = partOfCLIOptionsImage()
			fieldInfo["cpus"] = info
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true, CPUsSet: true}
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.InDelta(t, 0.0, res.CPUs, 1e-9)
	})

	t.Run("applyStringSliceOption with non-slice field", func(t *testing.T) {
		withPatchedFieldInfo(t, "dns", func() {
			info := fieldInfo["dns"]
			// Point p1ValIdx to a string field (Image)
			info.p1ValIdx = partOfCLIOptionsImage()
			fieldInfo["dns"] = info
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true, CderunDNS: []string{"8.8.8.8"}} // This will not be used because reflection will see String
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Empty(t, res.DNS)
	})

	t.Run("applyStringSliceOption p2 non-slice", func(t *testing.T) {
		withPatchedFieldInfo(t, "dns", func() {
			info := fieldInfo["dns"]
			info.p2ValIdx = partOfCLIOptionsImage()
			fieldInfo["dns"] = info
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true, DNS: []string{"8.8.8.8"}}
		res, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Empty(t, res.DNS)
	})

	t.Run("validateSecurity privileged warning", func(t *testing.T) {
		origLevel := logging.GetGlobalLogger().GetLevel()
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		defer logging.GetGlobalLogger().SetLevel(origLevel)

		cli := &CLIOptions{Image: "alpine", ImageSet: true, Privileged: true, PrivilegedSet: true}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
	})

	t.Run("validateEnvSecurity invalid key", func(t *testing.T) {
		// Test case where resolveEnvValues doesn't catch the invalid key because there's no "="
		// and it's not in strict mode.
		cli := &CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"BAD\nKEY"}}
		_, err := ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env[0] (key)")

		cli = &CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"-INVALID"}}
		_, err = ResolveWithFS("sh", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env[0] (key)")
	})

	t.Run("resolveStandardOptions registry mismatch on bool option", func(t *testing.T) {
		withPatchedFieldInfo(t, "tty", func() {
			delete(fieldInfo, "tty")
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"tty\" not found")
	})

	t.Run("applyIntOption registry mismatch", func(t *testing.T) {
		withPatchedFieldInfo(t, "pull-max-retries", func() {
			delete(fieldInfo, "pull-max-retries")
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"pull-max-retries\" not found")
	})

	t.Run("applyFloat64Option registry mismatch", func(t *testing.T) {
		withPatchedFieldInfo(t, "cpus", func() {
			delete(fieldInfo, "cpus")
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"cpus\" not found")
	})

	t.Run("applyStringSliceOption registry mismatch", func(t *testing.T) {
		withPatchedFieldInfo(t, "dns", func() {
			delete(fieldInfo, "dns")
		})

		cli := &CLIOptions{Image: "alpine", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option \"dns\" not found")
	})

	t.Run("validateSecurity exhaustive critical fields", func(t *testing.T) {
		testCases := []struct {
			name         string
			cli          CLIOptions
			wantErr      bool
			errContains  string
			wantLogLevel string
		}{
			{
				name: "Valid warning alias for log-level",
				cli: CLIOptions{
					Image: "alpine", ImageSet: true,
					LogLevel: "warning", LogLevelSet: true,
				},
				wantErr:      false,
				wantLogLevel: "warning",
			},
			{
				name: "Invalid log-level",
				cli: CLIOptions{
					Image: "alpine", ImageSet: true,
					LogLevel: "invalid", LogLevelSet: true,
				},
				wantErr:     true,
				errContains: "unsupported log level: \"invalid\"",
			},
			{
				name: "Invalid runtime",
				cli: CLIOptions{
					Image: "alpine", ImageSet: true,
					Runtime: "rkt", RuntimeSet: true,
				},
				wantErr:     true,
				errContains: "unsupported runtime: \"rkt\"",
			},
			{
				name: "Invalid dry-run-format",
				cli: CLIOptions{
					Image: "alpine", ImageSet: true,
					DryRunFormat: "xml", DryRunFormatSet: true,
				},
				wantErr:     true,
				errContains: "unsupported dry-run format: \"xml\"",
			},
			{
				name: "Invalid diagnosis-format",
				cli: CLIOptions{
					Image: "alpine", ImageSet: true,
					DiagnosisFormat: "xml", DiagnosisFormatSet: true,
				},
				wantErr:     true,
				errContains: "unsupported diagnosis format: \"xml\"",
			},
			{
				name: "Invalid log-format",
				cli: CLIOptions{
					Image: "alpine", ImageSet: true,
					LogFormat: "xml", LogFormatSet: true,
				},
				wantErr:     true,
				errContains: "unsupported log format: \"xml\"",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				res, err := ResolveWithFS("sh", &tc.cli, nil, nil, &MockFileSystem{})
				if tc.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.errContains)
				} else {
					require.NoError(t, err)
					if tc.wantLogLevel != "" {
						assert.Equal(t, tc.wantLogLevel, res.LogLevel)
					}
				}
			})
		}
	})
}

func partOfCLIOptionsImage() int {
	f, ok := reflect.TypeFor[CLIOptions]().FieldByName("Image")
	if !ok || len(f.Index) == 0 {
		panic("Field 'Image' not found in CLIOptions")
	}
	return f.Index[0]
}

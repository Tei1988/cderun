package config

import (
	"testing"
)

func BenchmarkResolveWithFS(b *testing.B) {
	mfs := &MockFileSystem{
		WD:      "/app/project/subdir",
		HomeDir: "/home/user",
		Env: map[string]string{
			"CDERUN_IMAGE":     "alpine",
			"CDERUN_TTY":       "true",
			"CDERUN_ENV":       "FOO=BAR; BAZ=QUX",
			"CDERUN_NETWORK":   "custom",
			"CDERUN_MOUNT":    "type=bind,source=/tmp,target=/tmp",
			"CDERUN_STRICT_ENV": "true",
		},
		Files: map[string][]byte{
			"/app/project/subdir/.version": []byte("1.2.3"),
		},
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image:   "node:20-alpine",
			Env:     []string{"NODE_ENV=production", "APP_VERSION={{file:.version}}"},
			Workdir: "/app",
			Ports:   []string{"3000:3000"},
		},
		"python": ToolConfig{
			Image: "python:3.11-slim",
		},
	}
	global := &CDERunConfig{
		Runtime: "docker",
		Defaults: ConfigDefaults{
			Network:     "bridge",
			Interactive: ptr(true),
		},
	}
	// Case 1: Subcommand with tool config and overrides
	cli := CLIOptions{
		Image:    "node:22",
		ImageSet: true,
		Env:      []string{"EXTRA=VAL"},
	}

	b.Run("SubcommandWithTool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ResolveWithFS("node", cli, tools, global, mfs)
		}
	})

	// Case 2: Polyglot/Symlink mode (subcommand matches tool name)
	b.Run("Polyglot", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ResolveWithFS("python", CLIOptions{}, tools, global, mfs)
		}
	})
}

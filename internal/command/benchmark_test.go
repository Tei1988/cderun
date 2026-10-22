package command

import (
	"io"
	"testing"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

func BenchmarkPreprocessArgs_StandardMode(b *testing.B) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	args := []string{"cderun", "--log-level", "info", "run", "--cderun-env", "FOO=BAR", "--cderun-memory", "1g", "arg1", "arg2"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := preprocessArgs(cmd, args)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkAssembleContainerCommand_Empty(b *testing.B) {
	passthroughArgs := []string{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := assembleContainerCommand(passthroughArgs)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkAssembleContainerCommand_NonEmpty(b *testing.B) {
	passthroughArgs := []string{"run", "--arg1", "val1", "--arg2", "val2"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := assembleContainerCommand(passthroughArgs)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkFormatDryRunSimple(b *testing.B) {
	cfg := &container.ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Network:     "bridge",
		Workdir:     "/app",
		User:        "1000:1000",
		TTY:         true,
		Env:         []string{"FOO=BAR", "BAZ=QUX"},
		Mounts:      []container.Mount{{Type: "bind", Source: "/host", Target: "/container", ReadOnly: true}},
		Ports:       []string{"8080:80"},
		Entrypoint:  []string{"/bin/sh", "-c"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = formatDryRunSimple(io.Discard, cfg)
	}
}

func BenchmarkBuildSnapshotHostContext(b *testing.B) {
	logger := logging.NewLogger()
	fs := config.RealFileSystem{}
	baseHostCtx := &config.HostContext{Level: 1}
	currentMounts := []container.Mount{
		{Type: "bind", Source: "/path/a", Target: "/target/a"},
		{Type: "bind", Source: "/path/b", Target: "/target/b"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = buildSnapshotHostContext(logger, fs, baseHostCtx, currentMounts, nil)
	}
}

func BenchmarkPreprocessArgs_PolyglotMode(b *testing.B) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	args := []string{"git", "status", "--cderun-env", "FOO=BAR", "--cderun-memory", "1g", "arg1", "arg2"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := preprocessArgs(cmd, args)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkBuildContainerConfig(b *testing.B) {
	opts := defaultOptions()
	resolved := &config.ResolvedConfig{
		Image:    "alpine:latest",
		Network:  "bridge",
		Workdir:  "/app",
		User:     "1000:1000",
		TTY:      true,
		Remove:   true,
		ReadOnly: true,
	}
	passthroughArgs := []string{"echo", "hello", "world"}
	toolsCfg := config.ToolsConfig{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := opts.buildContainerConfig(resolved, passthroughArgs, toolsCfg)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

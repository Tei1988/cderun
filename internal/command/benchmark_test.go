package command

import (
	"testing"

	"cderun/internal/config"
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

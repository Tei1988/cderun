package config

import (
	"testing"
)

func BenchmarkResolveWithFS(b *testing.B) {
	cli := CLIOptions{
		Image:    "alpine",
		ImageSet: true,
		TTY:      true,
		TTYSet:   true,
		Env:      []string{"VAR1=VAL1", "VAR2=VAL2"},
	}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20",
			Env:   []string{"TOOL_VAR=TOOL_VAL"},
		},
	}
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			Network: "bridge",
		},
	}
	mfs := &MockFileSystem{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ResolveWithFS("node", cli, tools, global, mfs)
	}
}

func BenchmarkExpressionResolver_ResolveString(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
	}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	input := "prefix-{{HOME}}-{{PWD}}-suffix"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.ResolveString(input)
	}
}

func BenchmarkExpressionResolver_ResolveString_NoExpr(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
	}
	r, _ := NewExpressionResolverWithFS(nil, mfs)
	input := "just-a-plain-string"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.ResolveString(input)
	}
}

package config

import (
	"testing"
)

func BenchmarkResolveWithFS(b *testing.B) {
	cli := CLIOptions{
		Image: Ptr("node:20"),
		TTY: Ptr(true),
		Env:      []string{"VAR1=VAL1", "VAR2=VAL2"}}
	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20",
			Env:   []string{"TOOL_VAR=TOOL_VAL"}}}
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			Network: "bridge"}}
	mfs := &MockFileSystem{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ResolveWithFS("node", &cli, tools, global, mfs)
		if err != nil {
			b.Fatalf("ResolveWithFS failed: %v", err)
		}
	}
}

func BenchmarkExpressionResolver_ResolveString(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	if err != nil {
		b.Fatalf("NewExpressionResolverWithFS failed: %v", err)
	}
	input := "prefix-{{HOME}}-{{PWD}}-suffix"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.ResolveString(input)
	}
}

func BenchmarkExpressionResolver_ResolveString_MagicOnly(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	if err != nil {
		b.Fatalf("NewExpressionResolverWithFS failed: %v", err)
	}
	input := "{{HOME}}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.ResolveString(input)
	}
}

func BenchmarkMaskSensitiveEnv(b *testing.B) {
	key := "DB_PASSWORD_SECRET_TOKEN"
	val := "my-secret-password"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MaskSensitiveEnv(key, val)
	}
}

func BenchmarkExpressionResolver_ResolveString_NoExpr(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	if err != nil {
		b.Fatalf("NewExpressionResolverWithFS failed: %v", err)
	}
	input := "just-a-plain-string"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.ResolveString(input)
	}
}

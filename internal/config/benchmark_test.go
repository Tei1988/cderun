package config

import (
	"testing"
)

func BenchmarkResolveWithFS(b *testing.B) {
	cli := CLIOptions{
		Image: ptr("node:20"),
		TTY:   ptr(true),
		Env:   []string{"VAR1=VAL1", "VAR2=VAL2"},
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
		_, err := ResolveWithFS("node", &cli, tools, global, mfs)
		if err != nil {
			b.Fatalf("ResolveWithFS failed: %v", err)
		}
	}
}

func BenchmarkExpressionResolver_ResolveString(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
	}
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

func BenchmarkExpressionResolver_ResolveString_MultipleExpressions(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
		Env: map[string]string{
			"USER": "testuser",
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	if err != nil {
		b.Fatalf("NewExpressionResolverWithFS failed: %v", err)
	}
	input := "prefix-{{HOME}}-{{PWD}}-{{env:USER:-default}}-suffix"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.ResolveString(input)
	}
}

func BenchmarkExpressionResolver_ResolveString_MagicOnly(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
	}
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
		_ = MaskSensitiveEnv(key, val, nil)
	}
}

func BenchmarkMaskSensitiveEnv_WithPatterns(b *testing.B) {
	b.Run("ExactMatch", func(b *testing.B) {
		b.ReportAllocs()
		key := "DB_PASSWORD"
		val := "my-secret-password"
		patterns := []string{"DB_PASSWORD", "DB_KEY", "DB_TOKEN"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnv(key, val, patterns)
		}
	})

	b.Run("UppercaseFastPath", func(b *testing.B) {
		b.ReportAllocs()
		key := "DB_PASSWORD_SECRET_TOKEN"
		val := "my-secret-password"
		patterns := []string{"*_TOKEN", "*_KEY", "*_PASSWORD"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnv(key, val, patterns)
		}
	})

	b.Run("MixedLowercaseConversion", func(b *testing.B) {
		b.ReportAllocs()
		key := "db_password_secret_token"
		val := "my-secret-password"
		patterns := []string{"*_token", "*_key", "*_password"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnv(key, val, patterns)
		}
	})
}

func BenchmarkMaskSensitiveEnvList_WithPatterns(b *testing.B) {
	b.Run("NilPatterns", func(b *testing.B) {
		b.ReportAllocs()
		env := []string{
			"DB_PASSWORD=my-secret-password",
			"DB_KEY=admin",
			"NORMAL_VAR=value",
			"ANOTHER_NORMAL_VAR=another-value",
			"PORT=8080",
			"HOST=localhost",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnvList(env, nil)
		}
	})

	b.Run("EmptyPatterns", func(b *testing.B) {
		b.ReportAllocs()
		env := []string{
			"DB_PASSWORD=my-secret-password",
			"DB_KEY=admin",
			"NORMAL_VAR=value",
			"ANOTHER_NORMAL_VAR=another-value",
			"PORT=8080",
			"HOST=localhost",
		}
		patterns := []string{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnvList(env, patterns)
		}
	})

	b.Run("ExactMatchLowercase", func(b *testing.B) {
		b.ReportAllocs()
		env := []string{
			"DB_PASSWORD=my-secret-password",
			"DB_KEY=admin",
			"NORMAL_VAR=value",
			"ANOTHER_NORMAL_VAR=another-value",
			"PORT=8080",
			"HOST=localhost",
		}
		patterns := []string{"db_password", "db_key", "db_token"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnvList(env, patterns)
		}
	})

	b.Run("ExactMatch", func(b *testing.B) {
		b.ReportAllocs()
		env := []string{
			"DB_PASSWORD=my-secret-password",
			"DB_KEY=admin",
			"NORMAL_VAR=value",
			"ANOTHER_NORMAL_VAR=another-value",
			"PORT=8080",
			"HOST=localhost",
		}
		patterns := []string{"DB_PASSWORD", "DB_KEY", "DB_TOKEN"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnvList(env, patterns)
		}
	})

	b.Run("UppercaseFastPath", func(b *testing.B) {
		b.ReportAllocs()
		env := []string{
			"DB_PASSWORD_SECRET_TOKEN=my-secret-password",
			"DB_USER_SECRET_KEY=admin",
			"NORMAL_VAR=value",
			"ANOTHER_NORMAL_VAR=another-value",
			"PORT=8080",
			"HOST=localhost",
		}
		patterns := []string{"*_TOKEN", "*_KEY", "*_PASSWORD"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnvList(env, patterns)
		}
	})

	b.Run("MixedLowercaseConversion", func(b *testing.B) {
		b.ReportAllocs()
		env := []string{
			"db_password_secret_token=my-secret-password",
			"db_user_secret_key=admin",
			"normal_var=value",
			"another_normal_var=another-value",
			"port=8080",
			"host=localhost",
		}
		patterns := []string{"*_token", "*_key", "*_password"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = MaskSensitiveEnvList(env, patterns)
		}
	})
}

func BenchmarkExpressionResolver_ResolveString_NoExpr(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
	}
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

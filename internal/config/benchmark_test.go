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

func BenchmarkValidatePathChars(b *testing.B) {
	b.Run("PrintableASCII", func(b *testing.B) {
		input := "type=bind,source=/home/user/project/subpath,target=/app/workdir,readonly"
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = validatePathChars(input)
		}
	})

	b.Run("ControlCharacter", func(b *testing.B) {
		input := "type=bind,source=/home/user/project/\x07subpath,target=/app"
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = validatePathChars(input)
		}
	})
}

func BenchmarkResolveWithFS_ComplexConfig(b *testing.B) {
	cli := CLIOptions{
		Image: ptr("node:20"),
		TTY:   ptr(true),
		Env:   []string{"VAR1=VAL1", "VAR2=VAL2", "VAR3=VAL3"},
		Mounts: []string{
			"type=bind,source=/host/path,target=/app",
			"type=tmpfs,target=/tmp",
		},
		Ulimits: []string{"nofile=1024:2048", "nproc=500:1000"},
		Sysctls: []string{"net.ipv4.ip_forward=1", "net.core.somaxconn=1024"},
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
	mfs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ResolveWithFS("node", &cli, tools, global, mfs)
		if err != nil {
			b.Fatalf("ResolveWithFS failed: %v", err)
		}
	}
}

func BenchmarkDeduplicateEnv(b *testing.B) {
	b.Run("SmallSlice", func(b *testing.B) {
		env := []string{
			"FOO=1", "BAR=2", "BAZ=3", "FOO=4", "QUX=5", "BAR=6", "ENV=7", "APP=8",
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = deduplicateEnv(env)
		}
	})

	b.Run("LargeSlice", func(b *testing.B) {
		env := []string{
			"VAR1=1", "VAR2=2", "VAR3=3", "VAR4=4", "VAR5=5",
			"VAR6=6", "VAR7=7", "VAR8=8", "VAR9=9", "VAR10=10",
			"VAR1=100", "VAR5=500", "VAR10=1000",
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = deduplicateEnv(env)
		}
	})
}

func BenchmarkExpressionResolver_ResolveString_MultipleExpressions(b *testing.B) {
	mfs := &MockFileSystem{
		HomeDir: "/home/user/very/long/path/for/benchmark/testing/directory/structure",
		WD:      "/app/workdir/path/for/benchmark/testing/directory/structure",
		Env: map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
			"VAR3": "val3",
			"VAR4": "val4",
			"VAR5": "val5",
			"VAR6": "val6",
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	if err != nil {
		b.Fatalf("NewExpressionResolverWithFS failed: %v", err)
	}
	input := "p1-{{HOME}}-p2-{{PWD}}-p3-{{env:VAR1}}-p4-{{env:VAR2}}-p5-{{env:VAR3}}-p6-{{env:VAR4}}-p7-{{env:VAR5}}-p8-{{env:VAR6}}-suffix"

	b.ReportAllocs()
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

package config

import (
	"bytes"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ValidateGroupAdd_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"basic group name", "wheel", false},
		{"basic GID", "1001", false},
		{"group name with trailing dollar", "domain_users$", false},
		{"invalid characters", "wheel-admin!", true},
		{"invalid spaces", "domain users", true},
		{"control character", "wheel\x01", true},
		{"command injection attempt", "wheel; rm -rf /", true},
		{"path traversal attempt", "wheel/../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGroupAdd(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateHostname_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"simple hostname", "web-server", false},
		{"localhost", "localhost", false},
		{"fqdn", "dev.local.host", false},
		{"invalid start with hyphen", "-web", true},
		{"invalid end with hyphen", "web-", true},
		{"invalid character underscore", "web_server", true},
		{"invalid character at-symbol", "@host", true},
		{"too long hostname", string(make([]byte, 254)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateNetworkName_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"standard network", "bridge", false},
		{"custom network with underscore and hyphen", "my_network-1", false},
		{"network name with dots", "custom-net.v1", false},
		{"invalid start with dot", ".net", true},
		{"invalid start with hyphen", "-net", true},
		{"invalid start with underscore", "_net", true},
		{"invalid special character", "net@work", true},
		{"command injection attempt", "bridge; echo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetworkName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateUserName_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"username root", "root", false},
		{"UID 1000", "1000", false},
		{"user with trailing dollar", "user_name$", false},
		{"user:group standard", "root:root", false},
		{"UID:GID standard", "1000:1001", false},
		{"trailing dollar for both parts", "user$:group$", false},
		{"too many colons", "root:root:root", true},
		{"invalid character in user", "user!name", true},
		{"invalid character in group", "user:group!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateDNS_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"IPv4 DNS", "8.8.8.8", false},
		{"IPv6 DNS", "2001:4860:4860::8888", false},
		{"invalid IPv4 segments", "999.999.999.999", true},
		{"too many segments", "1.1.1.1.1", true},
		{"non-IP hostname", "dns.google", true},
		{"control character", "8.8.8.8\x01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNS(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateAddHost_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"standard host gateway mapping", "host.docker.internal:host-gateway", false},
		{"standard host IP mapping", "myhost:192.168.1.1", false},
		{"invalid hostname part", "my_host:192.168.1.1", true},
		{"invalid IP part", "myhost:999.999.999.999", true},
		{"missing IP separator", "myhost", true},
		{"too many separators", "myhost:192.168.1.1:extra", true},
		{"control character", "myhost:192.168.1.1\x01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddHost(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateCapability_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"standard upper capability", "SYS_ADMIN", false},
		{"CAP-prefixed standard capability", "CAP_SYS_ADMIN", false},
		{"lowercase capability name", "sys_admin", true},
		{"special character hyphen", "SYS-ADMIN", true},
		{"command injection attempt", "SYS_ADMIN; rm", true},
		{"control character", "SYS_ADMIN\x01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapability(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateWorkdir_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"standard absolute path", "/app", false},
		{"complex absolute path", "/var/lib/docker/tmp", false},
		{"relative path", "app", true},
		{"space in path", "/app space", true},
		{"special character in path", "/app!", true},
		{"path with parent traversal", "/app/../etc", true},
		{"path ending with parent traversal", "/usr/..", true},
		{"path starting with parent traversal", "/../usr", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkdir(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateExposePort_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"single port", "80", false},
		{"single port with tcp", "80/tcp", false},
		{"single port with udp", "53/udp", false},
		{"port range", "1000-2000", false},
		{"port range with udp", "3000-4000/udp", false},
		{"negative port", "-80", true},
		{"port out of range", "65536", true},
		{"non-numeric port", "abc", true},
		{"invalid port range start > end", "2000-1000", true},
		{"invalid port range missing end", "3000-", true},
		{"invalid port range missing start", "-4000", true},
		{"invalid protocol", "80/http", true},
		{"control character", "80\x01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExposePort(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateToolName_Robustness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"basic name", "node", false},
		{"name with dots", "python3.11", false},
		{"name with underscores and hyphens", "go_tool-v1", false},
		{"empty string", "", true},
		{"absolute path", "/usr/bin/node", true},
		{"parent directory dotdot", "..", true},
		{"current directory dot", ".", true},
		{"special character", "tool!", true},
		{"colon in name", "tool:", true},
		{"space in name", "tool space", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateDeviceSecurity_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("device with control characters raises validation error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Devices: []string{"/dev/null\x01"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for devices")
	})

	t.Run("programmatic invalid permissions raises validation error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Devices: []DeviceConfig{
					{
						Source:      ConfigPath{Raw: "/dev/null"},
						Destination: ConfigPath{Raw: "/dev/null"},
						Permissions: "invalid",
					},
				},
			},
		}

		_, err := ResolveWithFS("sh", cli, tools, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cgroup permissions \"invalid\"")
	})
}

func TestUnit_Config_Resolver_FastPathAndFallbacks_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("empty options and nil config defaults to standard values", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		// Defaults from StringOptions/BoolOptions should be resolved
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)
		assert.False(t, res.TTY)
		assert.False(t, res.Interactive)
	})

	t.Run("invalid pull policy raises error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Pull:  ptr("invalid_policy"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy")
	})

	t.Run("invalid log level raises error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			LogLevel: ptr("VERBOSE"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log level")
	})

	t.Run("invalid log format raises error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:     ptr("alpine"),
			LogFormat: ptr("yaml"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log format")
	})
}

func TestUnit_Config_Resolver_DoubleBracesEscaping_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("resolver does not resolve doubly braced literal in environment value", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"MY_VAR": "resolved_value",
			},
		}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"MY_VAL={{ {{env:MY_VAR}} }}"},
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		// Escaped with double braces -> should preserve the inner expression as a literal string
		assert.Contains(t, res.Env, "MY_VAL={{env:MY_VAR}}")
	})
}

func TestUnit_Config_Resolver_NegativeDurationErrors_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("negative pull-backoff-base raises validation error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:           ptr("alpine"),
			PullBackoffBase: ptr("-10s"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "pull-backoff-base", cfgErr.Field)
	})

	t.Run("negative hang-timeout raises validation error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			HangTimeout: ptr("-5s"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "hang-timeout", cfgErr.Field)
	})
}

func TestUnit_Config_Resolver_PrecedenceRegistryMatching_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("registry check mismatches on option fetching returns mismatch error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}

		// Mock a registry mismatch error by calling inner resolver's applyStringOption with unknown options.
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)
		res := &ResolvedConfig{}
		rv := &resolver{
			subcommand: "sh",
			cli:        cli,
			fs:         mfs,
			r:          r,
			res:        res,
		}

		opt := StringOption{
			Name: "nonexistent-opt",
		}
		err = rv.applyStringOption(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch")
	})

	t.Run("registry check mismatches on boolean option fetching returns mismatch error", func(t *testing.T) {
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)
		res := &ResolvedConfig{}
		rv := &resolver{
			subcommand: "sh",
			r:          r,
			res:        res,
		}

		opt := BoolOption{
			Name: "nonexistent-opt",
		}
		err = rv.applyBoolOption(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch")
	})

	t.Run("registry check mismatches on integer option fetching returns mismatch error", func(t *testing.T) {
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)
		res := &ResolvedConfig{}
		rv := &resolver{
			subcommand: "sh",
			r:          r,
			res:        res,
		}

		opt := IntOption{
			Name: "nonexistent-opt",
		}
		err = rv.applyIntOption(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch")
	})

	t.Run("registry check mismatches on float64 option fetching returns mismatch error", func(t *testing.T) {
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)
		res := &ResolvedConfig{}
		rv := &resolver{
			subcommand: "sh",
			r:          r,
			res:        res,
		}

		opt := Float64Option{
			Name: "nonexistent-opt",
		}
		err = rv.applyFloat64Option(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch")
	})
}

func TestUnit_Config_Resolver_ResourceLimitsNegative_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("negative memory in CLI options is rejected", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Memory: ptr("9223372036854775808"), // Large value that causes overflow to negative int64
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid memory value \"-9223372036854775808\"")
	})

	t.Run("negative CPUs in CLI options is rejected", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			CPUs:  ptr(-2.5),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CPU limit cannot be negative")
	})
}

func TestUnit_Config_Resolver_PrivilegedCapWarnings_Robustness(t *testing.T) {
	// Not Parallel because it manipulates global logger output and level
	mfs := &MockFileSystem{}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("privileged mode with standard caps does not trigger additional cap warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:      ptr("alpine"),
			Privileged: ptr(true),
			CapAdd:     []string{"CHOWN", "MKNOD"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		// Should have standard privileged warning
		assert.Contains(t, logOutput, "Container is running in privileged mode")
		// Should NOT have highly privileged caps warning
		assert.NotContains(t, logOutput, "Highly privileged capability")
	})

	t.Run("privileged mode with highly privileged caps triggers cap warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:      ptr("alpine"),
			Privileged: ptr(true),
			CapAdd:     []string{"SYS_ADMIN", "NET_ADMIN"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		// Should have standard privileged warning
		assert.Contains(t, logOutput, "Container is running in privileged mode")
		// Should have highly privileged caps warning
		assert.Contains(t, logOutput, "Highly privileged capability")
		assert.Contains(t, logOutput, "SYS_ADMIN")
		assert.Contains(t, logOutput, "NET_ADMIN")
	})

	t.Run("non-privileged mode with highly privileged caps triggers cap warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:  ptr("alpine"),
			CapAdd: []string{"SYS_ADMIN", "NET_ADMIN"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Highly privileged capability")
		assert.Contains(t, logOutput, "SYS_ADMIN")
		assert.Contains(t, logOutput, "NET_ADMIN")
	})
}

func TestUnit_Config_Resolver_EnvKeyValidation_Robustness(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	tests := []struct {
		name      string
		env       []string
		strict    bool
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "Valid key-value environment variable",
			env:     []string{"VALID_KEY=value"},
			strict:  false,
			wantErr: false,
		},
		{
			name:    "Valid passthrough environment variable",
			env:     []string{"VALID_KEY"},
			strict:  false,
			wantErr: false,
		},
		{
			name:      "Invalid environment key with prefix number",
			env:       []string{"123_INVALID=value"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Invalid passthrough environment key with prefix number",
			env:       []string{"123_INVALID"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Invalid environment key with control char in key-value",
			env:       []string{"BAD\nKEY=value"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Invalid passthrough environment key with control char",
			env:       []string{"BAD\nKEY"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Invalid environment key with hyphen in key-value",
			env:       []string{"INVALID-KEY=value"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Invalid passthrough environment key with hyphen",
			env:       []string{"INVALID-KEY"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Empty environment variable string",
			env:       []string{""},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
		{
			name:      "Valid key and invalid key combined",
			env:       []string{"VALID_KEY=value", "INVALID-KEY"},
			strict:    false,
			wantErr:   true,
			errSubstr: "security validation failed for env[1] (key)",
		},
		{
			name:      "Invalid passthrough key is validated before strict lookup",
			env:       []string{"INVALID-KEY"},
			strict:    true,
			wantErr:   true,
			errSubstr: "security validation failed for env[0] (key)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &CLIOptions{
				Image: ptr("alpine"),
				Env:   tt.env,
			}
			if tt.strict {
				cli.StrictEnv = ptr(true)
			}

			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

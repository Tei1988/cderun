package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolveSysctls_SuccessAndPrecedence(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"CDERUN_SYSCTL": "net.ipv4.ip_forward=1,kernel.threads-max=2000",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Test 1: Env variable resolution with comma-separated list
	t.Run("CDERUN_SYSCTL env parsing and resolution", func(t *testing.T) {
		res, err := resolveSysctls(nil, nil, "sub", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"net.ipv4.ip_forward": "1",
			"kernel.threads-max":  "2000",
		}, res)
	})

	// Test 2: P2 CLI flag overrides env variable
	t.Run("P2 overrides Env", func(t *testing.T) {
		p2 := []string{"net.ipv4.tcp_syncookies=1"}
		res, err := resolveSysctls(nil, p2, "sub", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"net.ipv4.tcp_syncookies": "1",
		}, res)
	})

	// Test 3: P1 Override flag overrides P2 CLI flag
	t.Run("P1 overrides P2", func(t *testing.T) {
		p1 := []string{"kernel.pid_max=32768"}
		p2 := []string{"net.ipv4.tcp_syncookies=1"}
		res, err := resolveSysctls(p1, p2, "sub", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"kernel.pid_max": "32768",
		}, res)
	})

	// Test 4: Expression resolution in values
	t.Run("Expression resolution in values", func(t *testing.T) {
		p2 := []string{"custom.path={{PWD}}/sysctl.conf"}
		res, err := resolveSysctls(nil, p2, "sub", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"custom.path": "/work/sysctl.conf",
		}, res)
	})

	// Test 5: Global defaults fallback
	t.Run("Global defaults fallback", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Sysctls: []string{"net.core.somaxconn=1024"},
			},
		}
		// Clear env to fall back to global
		mfsEmptyEnv := &MockFileSystem{WD: "/work", HomeDir: "/home/user"}
		res, err := resolveSysctls(nil, nil, "sub", nil, global, r, mfsEmptyEnv)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"net.core.somaxconn": "1024",
		}, res)
	})

	// Test 6: Tool config fallback overrides global
	t.Run("Tool config overrides global", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Sysctls: []string{"net.core.somaxconn=1024"},
			},
		}
		tools := ToolsConfig{
			"sub": ToolConfig{
				Sysctls: []string{"net.core.somaxconn=2048"},
			},
		}
		mfsEmptyEnv := &MockFileSystem{WD: "/work", HomeDir: "/home/user"}
		res, err := resolveSysctls(nil, nil, "sub", tools, global, r, mfsEmptyEnv)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"net.core.somaxconn": "2048",
		}, res)
	})
}

func TestUnit_Config_ResolveSysctls_ValidationFailure(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	tests := []struct {
		name    string
		p2      []string
		wantErr string
	}{
		{
			name:    "Missing equals sign",
			p2:      []string{"net.ipv4.ip_forward"},
			wantErr: "invalid sysctl config",
		},
		{
			name:    "Multiple equals sign allowed if key is not empty",
			p2:      []string{"net.ipv4.ip_forward=1=2"},
			wantErr: "",
		},
		{
			name:    "Empty key",
			p2:      []string{"=1"},
			wantErr: "invalid sysctl config",
		},
		{
			name:    "Empty key after trim",
			p2:      []string{"  =1"},
			wantErr: "invalid sysctl config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveSysctls(nil, tt.p2, "sub", nil, nil, r, mfs)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestUnit_Config_ResolveSysctls_DeepCopy(t *testing.T) {
	t.Parallel()

	defaults := ConfigDefaults{
		Sysctls: []string{"net.ipv4.ip_forward=1"},
	}
	copied := defaults.DeepCopy()
	assert.Equal(t, defaults.Sysctls, copied.Sysctls)

	// Ensure slice is deeply copied and not sharing backing array
	copied.Sysctls[0] = "net.ipv4.ip_forward=0"
	assert.NotEqual(t, defaults.Sysctls[0], copied.Sysctls[0])

	tool := ToolConfig{
		Sysctls: []string{"net.ipv4.ip_forward=2"},
	}
	copiedTool := tool.DeepCopy()
	assert.Equal(t, tool.Sysctls, copiedTool.Sysctls)
	copiedTool.Sysctls[0] = "net.ipv4.ip_forward=0"
	assert.NotEqual(t, tool.Sysctls[0], copiedTool.Sysctls[0])
}

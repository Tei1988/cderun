package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Sysctl_ParsingAndResolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/app",
	}

	t.Run("resolve sysctls from CLI (P2)", func(t *testing.T) {
		res, err := resolveSysctls(nil, []string{"net.ipv4.ip_forward=1", "kernel.threads-max=100000"}, "", nil, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Equal(t, "1", res["net.ipv4.ip_forward"])
		assert.Equal(t, "100000", res["kernel.threads-max"])
	})

	t.Run("resolve sysctls from CLI override (P1 overrides P2)", func(t *testing.T) {
		res, err := resolveSysctls([]string{"net.ipv4.ip_forward=0"}, []string{"net.ipv4.ip_forward=1"}, "", nil, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "0", res["net.ipv4.ip_forward"])
	})

	t.Run("resolve sysctls from environment variable", func(t *testing.T) {
		mfs.Env = map[string]string{
			"CDERUN_SYSCTL": "net.ipv4.ip_forward=1,kernel.threads-max=100000",
		}
		defer func() { mfs.Env = nil }()

		res, err := resolveSysctls(nil, nil, "", nil, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Equal(t, "1", res["net.ipv4.ip_forward"])
		assert.Equal(t, "100000", res["kernel.threads-max"])
	})

	t.Run("resolve sysctls from tool configuration", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Sysctls: []string{"net.ipv4.ip_forward=1"},
			},
		}

		res, err := resolveSysctls(nil, nil, "node", tools, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "1", res["net.ipv4.ip_forward"])
	})

	t.Run("resolve sysctls from global defaults", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Sysctls: []string{"kernel.threads-max=50000"},
			},
		}

		res, err := resolveSysctls(nil, nil, "sh", nil, global, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "50000", res["kernel.threads-max"])
	})

	t.Run("resolve sysctls with dynamic expression resolution", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		mfs.Env = map[string]string{
			"FORWARD_VAL": "1",
		}
		defer func() { mfs.Env = nil }()

		res, err := resolveSysctls(nil, []string{"net.ipv4.ip_forward={{env:FORWARD_VAL}}"}, "", nil, nil, r, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "1", res["net.ipv4.ip_forward"])
	})

	t.Run("resolve sysctls with dynamic expression resolution in key", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		mfs.Env = map[string]string{
			"FORWARD_KEY": "net.ipv4.ip_forward",
		}
		defer func() { mfs.Env = nil }()

		res, err := resolveSysctls(nil, []string{"{{env:FORWARD_KEY}}=1"}, "", nil, nil, r, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "1", res["net.ipv4.ip_forward"])
	})

	t.Run("resolve sysctls with invalid format errors out", func(t *testing.T) {
		_, err := resolveSysctls(nil, []string{"invalid-format"}, "", nil, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Equal(t, "invalid-format", cfgErr.Value)
	})

	t.Run("resolve sysctls with empty key errors out", func(t *testing.T) {
		_, err := resolveSysctls(nil, []string{"=1"}, "", nil, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Equal(t, "=1", cfgErr.Value)
	})

	t.Run("resolve sysctls with null byte key injection errors out", func(t *testing.T) {
		_, err := resolveSysctls(nil, []string{"net.ipv4.ip_forward\x00=1"}, "", nil, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Equal(t, "net.ipv4.ip_forward\x00=1", cfgErr.Value)
		assert.Contains(t, cfgErr.Err.Error(), "null byte injection")
	})

	t.Run("resolve sysctls with null byte value injection errors out", func(t *testing.T) {
		_, err := resolveSysctls(nil, []string{"net.ipv4.ip_forward=1\x00"}, "", nil, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Equal(t, "net.ipv4.ip_forward=1\x00", cfgErr.Value)
		assert.Contains(t, cfgErr.Err.Error(), "null byte injection")
	})

	t.Run("resolve sysctls with resolution expression error propagates", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		r.setError(errors.New("sticky error"))
		_, err = resolveSysctls(nil, []string{"net.ipv4.ip_forward=1"}, "", nil, nil, r, mfs)
		require.Error(t, err)
		assert.Equal(t, "sticky error", err.Error())
	})
}

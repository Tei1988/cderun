package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolveOptionFieldInfo(t *testing.T) {
	fieldOnce.Do(initFieldInfo)

	t.Run("valid option field info resolution", func(t *testing.T) {
		expectedInfo := fieldInfo["network"]
		res, err := resolveOptionFieldInfo("network", expectedInfo)
		require.NoError(t, err)
		assert.Equal(t, expectedInfo, res)
	})

	t.Run("zero info falls back to fieldInfo map lookup", func(t *testing.T) {
		zero := optionFields{targetIdx: 0, p1ValIdx: 0, p2ValIdx: 0}
		res, err := resolveOptionFieldInfo("network", zero)
		require.NoError(t, err)
		expectedInfo := fieldInfo["network"]
		assert.Equal(t, expectedInfo, res)
	})

	t.Run("unknown option name returns error when fallback lookup fails", func(t *testing.T) {
		zero := optionFields{targetIdx: 0, p1ValIdx: 0, p2ValIdx: 0}
		_, err := resolveOptionFieldInfo("nonexistent_option_name_xyz", zero)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry mismatch: info for option")
	})
}

func TestUnit_Config_IsDriftOk(t *testing.T) {
	fieldOnce.Do(initFieldInfo)

	t.Run("matches expectedFieldIndices", func(t *testing.T) {
		expected, ok := expectedFieldIndices["network"]
		require.True(t, ok, "expectedFieldIndices should contain 'network' entry")
		assert.True(t, isDriftOk("network", expected))
	})

	t.Run("drift mismatch returns false", func(t *testing.T) {
		mismatched := optionFields{targetIdx: 999, p1ValIdx: -1, p2ValIdx: -1}
		assert.False(t, isDriftOk("network", mismatched))
	})
}

func TestUnit_Config_OptionFastPathHelpers(t *testing.T) {
	t.Run("getStringOptionFastPathPtrs and assignResolvedString", func(t *testing.T) {
		valP1 := "p1-val"
		valP2 := "p2-val"
		cli := &CLIOptions{
			CderunNetwork: &valP1,
			Network:       &valP2,
		}

		p1Ptr, p2Ptr, ok := getStringOptionFastPathPtrs(cli, "network")
		require.True(t, ok)
		assert.Equal(t, &valP1, p1Ptr)
		assert.Equal(t, &valP2, p2Ptr)

		_, _, okNonExistent := getStringOptionFastPathPtrs(cli, "unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		assignResolvedString(res, "network", "host")
		assert.Equal(t, "host", res.Network)
	})

	t.Run("getBoolOptionFastPathPtrs and assignResolvedBool", func(t *testing.T) {
		boolTrue := true
		boolFalse := false
		cli := &CLIOptions{
			CderunTTY: &boolTrue,
			TTY:       &boolFalse,
		}

		p1Ptr, p2Ptr, ok := getBoolOptionFastPathPtrs(cli, "tty")
		require.True(t, ok)
		assert.Equal(t, &boolTrue, p1Ptr)
		assert.Equal(t, &boolFalse, p2Ptr)

		_, _, okNonExistent := getBoolOptionFastPathPtrs(cli, "unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		assignResolvedBool(res, "tty", true)
		assert.True(t, res.TTY)
	})

	t.Run("getStringSliceOptionFastPathSlices and assignResolvedStringSlice", func(t *testing.T) {
		p1Slice := []string{"8080:8080"}
		p2Slice := []string{"9090:9090"}
		cli := &CLIOptions{
			CderunPorts: p1Slice,
			Ports:       p2Slice,
		}

		s1, s2, ok := getStringSliceOptionFastPathSlices(cli, "publish")
		require.True(t, ok)
		assert.Equal(t, p1Slice, s1)
		assert.Equal(t, p2Slice, s2)

		_, _, okNonExistent := getStringSliceOptionFastPathSlices(cli, "unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		assignResolvedStringSlice(res, "publish", []string{"80:80"})
		assert.Equal(t, []string{"80:80"}, res.Ports)
	})

	t.Run("getIntOptionFastPathPtrs and assignResolvedInt", func(t *testing.T) {
		val1 := 5
		val2 := 3
		cli := &CLIOptions{
			CderunPullMaxRetries: &val1,
			PullMaxRetries:       &val2,
		}

		p1Ptr, p2Ptr, ok := getIntOptionFastPathPtrs(cli, "pull-max-retries")
		require.True(t, ok)
		assert.Equal(t, &val1, p1Ptr)
		assert.Equal(t, &val2, p2Ptr)

		_, _, okNonExistent := getIntOptionFastPathPtrs(cli, "unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		assignResolvedInt(res, "pull-max-retries", 10)
		assert.Equal(t, 10, res.PullMaxRetries)
	})

	t.Run("getFloat64OptionFastPathPtrs and assignResolvedFloat64", func(t *testing.T) {
		val1 := 2.5
		val2 := 1.0
		cli := &CLIOptions{
			CderunCPUs: &val1,
			CPUs:       &val2,
		}

		p1Ptr, p2Ptr, ok := getFloat64OptionFastPathPtrs(cli, "cpus")
		require.True(t, ok)
		assert.Equal(t, &val1, p1Ptr)
		assert.Equal(t, &val2, p2Ptr)

		_, _, okNonExistent := getFloat64OptionFastPathPtrs(cli, "unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		assignResolvedFloat64(res, "cpus", 4.0)
		assert.Equal(t, 4.0, res.CPUs)
	})
}

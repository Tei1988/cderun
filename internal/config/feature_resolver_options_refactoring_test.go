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
	t.Run("getStringOptionFastPathPtrsAndAssign", func(t *testing.T) {
		valP1 := "p1-val"
		valP2 := "p2-val"
		cli := &CLIOptions{
			CderunNetwork: &valP1,
			Network:       &valP2,
		}

		p1Get, p2Assign, ok := getStringOptionFastPathPtrsAndAssign("network")
		require.True(t, ok)
		p1Ptr, p2Ptr, okGet := p1Get(cli)
		assert.True(t, okGet)
		assert.Equal(t, &valP1, p1Ptr)
		assert.Equal(t, &valP2, p2Ptr)

		_, _, okNonExistent := getStringOptionFastPathPtrsAndAssign("unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		p2Assign(res, "host")
		assert.Equal(t, "host", res.Network)
	})

	t.Run("getBoolOptionFastPathPtrsAndAssign", func(t *testing.T) {
		boolTrue := true
		boolFalse := false
		cli := &CLIOptions{
			CderunTTY: &boolTrue,
			TTY:       &boolFalse,
		}

		p1Get, p2Assign, ok := getBoolOptionFastPathPtrsAndAssign("tty")
		require.True(t, ok)
		p1Ptr, p2Ptr, okGet := p1Get(cli)
		assert.True(t, okGet)
		assert.Equal(t, &boolTrue, p1Ptr)
		assert.Equal(t, &boolFalse, p2Ptr)

		_, _, okNonExistent := getBoolOptionFastPathPtrsAndAssign("unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		p2Assign(res, true)
		assert.True(t, res.TTY)
	})

	t.Run("getStringSliceOptionFastPathSlicesAndAssign", func(t *testing.T) {
		p1Slice := []string{"8080:8080"}
		p2Slice := []string{"9090:9090"}
		cli := &CLIOptions{
			CderunPorts: p1Slice,
			Ports:       p2Slice,
		}

		p1Get, p2Assign, ok := getStringSliceOptionFastPathSlicesAndAssign("publish")
		require.True(t, ok)
		s1, s2, okGet := p1Get(cli)
		assert.True(t, okGet)
		assert.Equal(t, p1Slice, s1)
		assert.Equal(t, p2Slice, s2)

		_, _, okNonExistent := getStringSliceOptionFastPathSlicesAndAssign("unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		p2Assign(res, []string{"80:80"})
		assert.Equal(t, []string{"80:80"}, res.Ports)
	})

	t.Run("getIntOptionFastPathPtrsAndAssign", func(t *testing.T) {
		val1 := 5
		val2 := 3
		cli := &CLIOptions{
			CderunPullMaxRetries: &val1,
			PullMaxRetries:       &val2,
		}

		p1Get, p2Assign, ok := getIntOptionFastPathPtrsAndAssign("pull-max-retries")
		require.True(t, ok)
		p1Ptr, p2Ptr, okGet := p1Get(cli)
		assert.True(t, okGet)
		assert.Equal(t, &val1, p1Ptr)
		assert.Equal(t, &val2, p2Ptr)

		_, _, okNonExistent := getIntOptionFastPathPtrsAndAssign("unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		p2Assign(res, 10)
		assert.Equal(t, 10, res.PullMaxRetries)
	})

	t.Run("getFloat64OptionFastPathPtrsAndAssign", func(t *testing.T) {
		val1 := 2.5
		val2 := 1.0
		cli := &CLIOptions{
			CderunCPUs: &val1,
			CPUs:       &val2,
		}

		p1Get, p2Assign, ok := getFloat64OptionFastPathPtrsAndAssign("cpus")
		require.True(t, ok)
		p1Ptr, p2Ptr, okGet := p1Get(cli)
		assert.True(t, okGet)
		assert.Equal(t, &val1, p1Ptr)
		assert.Equal(t, &val2, p2Ptr)

		_, _, okNonExistent := getFloat64OptionFastPathPtrsAndAssign("unknown-opt")
		assert.False(t, okNonExistent)

		res := &ResolvedConfig{}
		p2Assign(res, 4.0)
		assert.Equal(t, 4.0, res.CPUs)
	})
}

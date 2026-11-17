package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type customEnvType struct {
	Val string
}

func TestUnit_ResolverHelpers_ParseEnvItemAndPickConfigs(t *testing.T) {
	t.Parallel()

	t.Run("parseEnvItem string type without parser", func(t *testing.T) {
		t.Parallel()
		item, err := parseEnvItem[string]("hello", nil)
		require.NoError(t, err)
		assert.Equal(t, "hello", item)
	})

	t.Run("parseEnvItem non-string type without parser returns error", func(t *testing.T) {
		t.Parallel()
		_, err := parseEnvItem[customEnvType]("hello", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parser required for non-string types")
	})

	t.Run("parseEnvItem with custom parser", func(t *testing.T) {
		t.Parallel()
		parser := func(s, src string) (customEnvType, error) {
			if s == "invalid" {
				return customEnvType{}, fmt.Errorf("invalid value in %s", src)
			}
			return customEnvType{Val: s + "_" + src}, nil
		}

		item, err := parseEnvItem("valid", parser)
		require.NoError(t, err)
		assert.Equal(t, customEnvType{Val: "valid_env"}, item)

		_, err = parseEnvItem("invalid", parser)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid value in env")
	})

	t.Run("pickConfigs single env item without separator", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Env: map[string]string{
				"TEST_ENV_SINGLE": "  single_value  ",
			},
		}

		res, err := pickConfigs[string](
			nil, nil, "TEST_ENV_SINGLE", ",", "", nil,
			nil, nil, nil, nil, mfs,
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"single_value"}, res)
	})

	t.Run("pickConfigs separated env items with whitespace", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Env: map[string]string{
				"TEST_ENV_SEP": " item1 , item2 ,, item3 ",
			},
		}

		res, err := pickConfigs[string](
			nil, nil, "TEST_ENV_SEP", ",", "", nil,
			nil, nil, nil, nil, mfs,
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"item1", "item2", "item3"}, res)
	})

	t.Run("pickConfigs empty env string returns empty slice", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Env: map[string]string{
				"TEST_ENV_EMPTY": "   ",
			},
		}

		res, err := pickConfigs[string](
			nil, nil, "TEST_ENV_EMPTY", ",", "", nil,
			nil, nil, nil, nil, mfs,
		)
		require.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("pickConfigs parser error propagation in separated items", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Env: map[string]string{
				"TEST_ENV_ERR": "valid1,bad_item,valid2",
			},
		}

		parser := func(s, src string) (customEnvType, error) {
			if s == "bad_item" {
				return customEnvType{}, errors.New("parse failed for bad_item")
			}
			return customEnvType{Val: s}, nil
		}

		_, err := pickConfigs(
			nil, nil, "TEST_ENV_ERR", ",", "", nil,
			nil, nil, nil, parser, mfs,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse failed for bad_item")
	})
}

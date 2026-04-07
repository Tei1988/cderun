package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_FieldInfo_ErrorPaths(t *testing.T) {
	cliVal := reflect.ValueOf(&CLIOptions{}).Elem()

	t.Run("missing registry info", func(t *testing.T) {
		_, _, _, _, _, err := fetchFieldAndParams("nonexistent", cliVal, fieldInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "info for option \"nonexistent\" not found")
	})

	t.Run("missing reflection fields", func(t *testing.T) {
		// Mock fieldInfo by adding an entry with missing ValIdx
		fieldOnce.Do(initFieldInfo)
		orig, ok := fieldInfo["image"]
		defer func() {
			if ok {
				fieldInfo["image"] = orig
			} else {
				delete(fieldInfo, "image")
			}
		}()

		fieldInfo["image"] = optionFields{p1ValIdx: nil}
		_, _, _, _, _, err := fetchFieldAndParams("image", cliVal, fieldInfo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CLI reflection fields for option \"image\" missing")
	})
}

func TestUnit_Config_GetFieldInfo_Exhaustive(t *testing.T) {
	type testStruct struct {
		Ptr       *string
		Interface any
		Map       map[string]string
		Zero      int
	}
	s := "test"
	ts := testStruct{
		Ptr:       &s,
		Interface: s,
		Map:       map[string]string{"a": "b"},
		Zero:      0,
	}
	val := reflect.ValueOf(ts)

	t.Run("Ptr set", func(t *testing.T) {
		set, _ := getFieldInfo(val, nil, []int{0})
		assert.True(t, set)
	})

	t.Run("Interface set", func(t *testing.T) {
		set, _ := getFieldInfo(val, nil, []int{1})
		assert.True(t, set)
	})

	t.Run("Map set", func(t *testing.T) {
		set, _ := getFieldInfo(val, nil, []int{2})
		assert.True(t, set)
	})

	t.Run("Zero not set", func(t *testing.T) {
		set, _ := getFieldInfo(val, nil, []int{3})
		assert.False(t, set)
	})

	tsNil := testStruct{
		Ptr:       nil,
		Interface: nil,
		Map:       nil,
	}
	valNil := reflect.ValueOf(tsNil)

	t.Run("Ptr nil", func(t *testing.T) {
		set, _ := getFieldInfo(valNil, nil, []int{0})
		assert.False(t, set)
	})

	t.Run("Interface nil", func(t *testing.T) {
		set, _ := getFieldInfo(valNil, nil, []int{1})
		assert.False(t, set)
	})

	t.Run("Map nil", func(t *testing.T) {
		set, _ := getFieldInfo(valNil, nil, []int{2})
		assert.False(t, set)
	})
}

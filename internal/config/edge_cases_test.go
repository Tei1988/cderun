package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Config_Option_Manual_Type_Mismatch(t *testing.T) {
	t.Run("resolveIntOpt with invalid env", func(t *testing.T) {
		def := OptionDef[*int]{
			EnvKey:   "TEST_INT",
			Fallback: ptr(10),
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "not-an-int"}}
		_, err := resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveFloat64Opt with invalid env", func(t *testing.T) {
		def := OptionDef[*float64]{
			EnvKey:   "TEST_FLOAT",
			Fallback: ptr(1.5),
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "not-a-float"}}
		_, err := resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveIntOpt with invalid env but valid ToolGetter", func(t *testing.T) {
		toolVal := 42
		def := OptionDef[*int]{
			EnvKey:     "TEST_INT",
			ToolGetter: func(tc ToolConfig) *int { return &toolVal },
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "invalid"}}
		_, err := resolveIntOpt(def, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveFloat64Opt with invalid env but valid GlobalGetter", func(t *testing.T) {
		globalVal := 2.5
		def := OptionDef[*float64]{
			EnvKey:       "TEST_FLOAT",
			GlobalGetter: func(c CDERunConfig) *float64 { return &globalVal },
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "invalid"}}
		_, err := resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, &CDERunConfig{}, mfs)
		assert.Error(t, err)
	})
}

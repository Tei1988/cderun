package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Config_ValidatePort_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Too many segments", "127.0.0.1:80:80:80", true},
		{"Empty segments (3rd)", "127.0.0.1:80:", true},
		{"Empty segments (2nd)", "127.0.0.1::80", false}, // documents current behavior: empty host port is accepted
		{"Empty IP in 3-segment", ":80:80", true},
		{"Invalid IP in 3-segment", "999.999.999.999:80:80", true},
		{"Non-numeric container port (3-segment)", "127.0.0.1:80:abc", true},
		{"Non-numeric host port (3-segment)", "127.0.0.1:abc:80", true},
		{"Non-numeric container port (2-segment)", "127.0.0.1:abc", true},
		{"Non-numeric host port (2-segment)", "abc:80", true},
		{"Non-numeric container port (1-segment)", "abc", true},
		{"Both segments empty", ":", true}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateExposePort_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Multi-dash range", "80-90-100", true},
		{"Empty start range", "-90", true},
		{"Empty end range", "80-", true},
		{"Non-numeric start range", "abc-90", true},
		{"Non-numeric end range", "80-abc", true},
		{"Invalid protocol", "80/http", true},
		{"Malformed protocol part", "80/", true}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExposePort(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_Masking_Advanced(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{"Acronym JSONToken", "JSONToken", "secret", "[REDACTED]"},
		{"Acronym APIKey", "APIKey", "secret", "[REDACTED]"},
		{"camelCase with digits db1Password", "db1Password", "secret", "[REDACTED]"},
		{"Snake case with Unicode ユーザー_TOKEN", "ユーザー_TOKEN", "secret", "[REDACTED]"},
		{"Boundary split acronym transition APIKeyExample", "APIKeyExample", "secret", "[REDACTED]"}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveEnv(tt.key, tt.value)
			assert.Equal(t, tt.expected, got, "key: %s", tt.key)
		})
	}
}

func TestUnit_Config_Option_Manual_Type_Mismatch(t *testing.T) {
	t.Run("resolveIntOpt with invalid env", func(t *testing.T) {
		def := OptionDef[*int]{
			EnvKey:   "TEST_INT",
			Fallback: Ptr(10)}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "not-an-int"}}
		res := resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Equal(t, 10, res)
	})

	t.Run("resolveFloat64Opt with invalid env", func(t *testing.T) {
		def := OptionDef[*float64]{
			EnvKey:   "TEST_FLOAT",
			Fallback: Ptr(1.5)}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "not-a-float"}}
		res := resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.InDelta(t, 1.5, res, 1e-9)
	})

	t.Run("resolveIntOpt with invalid env but valid ToolGetter", func(t *testing.T) {
		toolVal := 42
		def := OptionDef[*int]{
			EnvKey:     "TEST_INT",
			ToolGetter: func(tc ToolConfig) *int { return &toolVal },
			Fallback:   Ptr(10)}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "not-an-int"}}
		tools := ToolsConfig{"sub": ToolConfig{}}
		// Env is invalid, so it should proceed to ToolGetter (P4) which is valid.
		res := resolveIntOpt(def, false, 0, false, 0, "sub", tools, nil, mfs)
		assert.Equal(t, 42, res)
	})

	t.Run("resolveFloat64Opt with invalid env but valid GlobalGetter", func(t *testing.T) {
		globalVal := 3.14
		def := OptionDef[*float64]{
			EnvKey:       "TEST_FLOAT",
			GlobalGetter: func(c CDERunConfig) *float64 { return &globalVal },
			Fallback:     Ptr(1.5)}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "not-a-float"}}
		// Env is invalid, ToolGetter is nil, so it should proceed to GlobalGetter (P5) which is valid.
		res := resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, &CDERunConfig{}, mfs)
		assert.InDelta(t, 3.14, res, 1e-9)
	})
}

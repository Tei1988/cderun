package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"image", "Image"},
		{"dry-run-format", "DryRunFormat"},
		{"tty", "TTY"},
		{"dns", "DNS"},
		{"cpus", "CPUs"},
		{"pull-backoff-base", "PullBackoffBase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, PascalCase(tt.input))
		})
	}
}

func TestGetOptions(t *testing.T) {
	t.Run("GetStringOption", func(t *testing.T) {
		opt, ok := GetStringOption("image")
		assert.True(t, ok)
		assert.Equal(t, "image", opt.Name)
		assert.Equal(t, "Image", opt.FieldName)

		_, ok = GetStringOption("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetBoolOption", func(t *testing.T) {
		opt, ok := GetBoolOption("tty")
		assert.True(t, ok)
		assert.Equal(t, "tty", opt.Name)
		assert.Equal(t, "TTY", opt.FieldName)

		_, ok = GetBoolOption("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetIntOption", func(t *testing.T) {
		opt, ok := GetIntOption("pull-max-retries")
		assert.True(t, ok)
		assert.Equal(t, "pull-max-retries", opt.Name)

		_, ok = GetIntOption("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetFloat64Option", func(t *testing.T) {
		opt, ok := GetFloat64Option("cpus")
		assert.True(t, ok)
		assert.Equal(t, "cpus", opt.Name)

		_, ok = GetFloat64Option("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetStringSliceOption", func(t *testing.T) {
		opt, ok := GetStringSliceOption("env")
		assert.True(t, ok)
		assert.Equal(t, "env", opt.Name)

		_, ok = GetStringSliceOption("nonexistent")
		assert.False(t, ok)
	})
}

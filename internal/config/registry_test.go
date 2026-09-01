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
		{"äpple", "Äpple"},
		{"üä-öß", "ÜäÖß"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, PascalCase(tt.input))
		})
	}
}

func TestRegistryOptionUsages(t *testing.T) {
	t.Run("sensitive-env usage description", func(t *testing.T) {
		opt, ok := GetStringSliceOption("sensitive-env")
		assert.True(t, ok)
		assert.Equal(t, "List of environment variable patterns to mask (default masks all variables)", opt.Usage)
	})

	t.Run("runtime usage description", func(t *testing.T) {
		opt, ok := GetStringOption("runtime")
		assert.True(t, ok)
		assert.Equal(t, "Container runtime to use (docker/podman/containerd)", opt.Usage)
	})
}

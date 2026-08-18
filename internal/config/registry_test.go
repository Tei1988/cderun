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

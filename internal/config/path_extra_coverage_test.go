package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_Path_ValidateImageName_Extra(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"Invalid start character", "_alpine", true},
		{"Invalid middle character", "alpine!", true},
		{"Multiple @ symbols", "alpine@sha256:abc@def", true},
		{"Valid with @", "alpine@sha256:abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageName(tt.image)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Path_MountConfig_UnmarshalYAML_Extra(t *testing.T) {
	t.Run("Decoding error", func(t *testing.T) {
		var mc MountConfig
		// Provide a non-mapping node (sequence) to trigger Decode error into struct
		err := yaml.Unmarshal([]byte("- item"), &mc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config")
	})
}

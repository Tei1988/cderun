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

func TestUnit_Path_Validators_Exhaustive(t *testing.T) {
	t.Run("ValidateEnvKey", func(t *testing.T) {
		tests := []struct {
			key     string
			wantErr bool
		}{
			{"VALID", false},
			{"_VALID", false},
			{"VALID_1", false},
			{"", true},
			{"1INVALID", true},
			{"INVALID!", true},
			{"INVALID-KEY", true},
		}
		for _, tt := range tests {
			err := ValidateEnvKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err, "key: %q", tt.key)
			} else {
				assert.NoError(t, err, "key: %q", tt.key)
			}
		}
	})

	t.Run("ValidatePort_Exhaustive", func(t *testing.T) {
		tests := []struct {
			port    string
			wantErr bool
		}{
			{"80", false},
			{"80:80", false},
			{"127.0.0.1:80", false},
			{"127.0.0.1:80:80", false},
			{"80/tcp", false},
			{"80/udp", false},
			{"", false},
			{"80/http", true},
			{"127.0.0.1:80:80:80", true},
			{"invalid", true},
			{"80:invalid", true},
			{"ip:80", true},
			{"127.0.0.1:invalid:80", true},
			{"127.0.0.1::80", false}, // empty hostPort is allowed in some runtimes, but let's see what our validator does.
		}
		for _, tt := range tests {
			err := ValidatePort(tt.port)
			if tt.wantErr {
				assert.Error(t, err, "port: %q", tt.port)
			} else {
				assert.NoError(t, err, "port: %q", tt.port)
			}
		}
	})

	t.Run("ValidateWorkdir_Exhaustive", func(t *testing.T) {
		tests := []struct {
			path    string
			wantErr bool
		}{
			{"/app", false},
			{"/app/src", false},
			{"/app_1.2-3", false},
			{"", false},
			{"relative", true},
			{"/app!", true},
			{"/app space", true},
		}
		for _, tt := range tests {
			err := ValidateWorkdir(tt.path)
			if tt.wantErr {
				assert.Error(t, err, "path: %q", tt.path)
			} else {
				assert.NoError(t, err, "path: %q", tt.path)
			}
		}
	})

	t.Run("ValidateToolName_Exhaustive", func(t *testing.T) {
		tests := []struct {
			name    string
			wantErr bool
		}{
			{"node", false},
			{"python3.11", false},
			{"go_tool-v1", false},
			{"", true},
			{"/usr/bin/node", true},
			{"..", true},
			{".", true},
			{"tool!", true},
			{"tool:", true},
			{"tool space", true},
		}
		for _, tt := range tests {
			err := ValidateToolName(tt.name)
			if tt.wantErr {
				assert.Error(t, err, "name: %q", tt.name)
			} else {
				assert.NoError(t, err, "name: %q", tt.name)
			}
		}
	})

	t.Run("isNamedVolume_Exhaustive", func(t *testing.T) {
		assert.True(t, isNamedVolume("my-vol"))
		assert.False(t, isNamedVolume(""))
		assert.False(t, isNamedVolume("/path"))
		assert.False(t, isNamedVolume("./path"))
		assert.False(t, isNamedVolume("~/path"))
		assert.False(t, isNamedVolume("C:\\path"))
	})
}

func TestUnit_Path_Resolve_Errors_Extra(t *testing.T) {
	t.Run("DeviceConfig UnmarshalYAML invalid kind", func(t *testing.T) {
		var dc DeviceConfig
		err := yaml.Unmarshal([]byte("- [seq]"), &dc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config")
	})

	t.Run("ResolvePath expandHome error", func(t *testing.T) {
		mfs := &pathMockFS{
			MockFileSystem: MockFileSystem{WD: "/base"},
			absErr:         assert.AnError,
		}
		mfs.HomeDir = "" // This might not trigger error in expandHome if it uses UserHomeDir
		// Let's use a specialized mock
		mfs2 := &exprMockFS{
			homeDirErr: assert.AnError,
		}
		// But we can pass r with a mock fs.
		r, _ := NewExpressionResolverWithFS(nil, mfs2)
		// Force ResolveString to return ~ by using an env var or similar
		mfs2.Env = map[string]string{"HOME_VAL": "~/foo"}
		_, err := ResolvePath("{{env:HOME_VAL}}", "/base", r)
		require.Error(t, err)
	})

	t.Run("ValidatePathChars", func(t *testing.T) {
		err := ValidatePathChars("path\x01")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("resolveVolumePath errors", func(t *testing.T) {
		mfs := &exprMockFS{
			readFileErr: assert.AnError,
		}
		r, _ := NewExpressionResolverWithFS(nil, mfs)
		r.setError(assert.AnError)
		// Case: host:remainder, but ResolveString(host) fails
		_, err := resolveVolumePath("{{file:err}}:/container", "/base", r)
		require.Error(t, err)

		// Case: no separator, ResolveString(v) fails
		_, err = resolveVolumePath("{{file:err}}", "/base", r)
		require.Error(t, err)
	})

	t.Run("validateAnchorBoundaries errors", func(t *testing.T) {
		mfs := &pathMockFS{
			absErr: assert.AnError,
		}
		// trigger fs.Abs(resolved) error
		err := validateAnchorBoundaries("~", "relative", nil, mfs)
		require.Error(t, err)

		// trigger ResolveString error for anchor
		r, _ := NewExpressionResolverWithFS(nil, &MockFileSystem{HomeDir: "/home"})
		r.setError(assert.AnError)
		err = validateAnchorBoundaries("{{err}}", "/abs", r, &MockFileSystem{})
		require.Error(t, err)
	})
}

package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ContainsNumericGID_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected bool
	}{
		{"nil slice", nil, false},
		{"empty slice", []string{}, false},
		{"purely non-numeric strings", []string{"sudo", "docker"}, false},
		{"purely numeric GID", []string{"1000"}, true},
		{"zero GID", []string{"0"}, true},
		{"partially numeric strings", []string{"1000a", "sudo"}, false},
		{"mixed slice with numeric GID", []string{"docker", "1001", "sudo"}, true},
		{"mixed slice without numeric GID", []string{"docker", "sudo123"}, false},
		{"empty string inside slice", []string{"", "docker"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ContainsNumericGID(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestUnit_Config_ValidateExposePort_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"standard single port", "80", false},
		{"port with protocol", "80/tcp", false},
		{"port with udp protocol", "53/udp", false},
		{"port range", "80-90", false},
		{"port range with protocol", "80-90/tcp", false},
		{"maximum single port", "65535", false},
		{"maximum port range with protocol", "1-65535/udp", false},
		{"empty exposed port", "", false},
		{"normal range and protocol", "80-90/udp", false},

		{"port zero", "0", true},
		{"out of bounds port", "70000", true},
		{"negative port", "-1", true},
		{"invalid protocol", "80/http", true},
		{"port range reversed start > end", "90-80", true},
		{"port range invalid start", "abc-80", true},
		{"port range invalid end", "80-abc", true},
		{"control character injection", "80\n", true},
		{"shell command injection", "80;rm", true},
		{"multiple protocols", "80/tcp/udp", true},
		{"port is negative", "-80", true},
		{"port range end negative", "80--90", true},
		{"port range end less than start", "100-90", true},
		{"invalid start port in range", "abc-90", true},
		{"invalid trailing hyphen", "80-", true},
		{"invalid leading hyphen", "-90", true},
		{"multiple hyphens", "80-90-100", true},
	}

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

func TestUnit_Config_ValidateNetworkName_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"standard bridge network", "bridge", false},
		{"network with hyphen", "my-net", false},
		{"network with dot", "my.net", false},
		{"network with underscore", "my_net", false},
		{"network with numbers", "net123", false},
		{"empty network name", "", false},

		{"starts with hyphen", "-net", true},
		{"starts with dot", ".net", true},
		{"starts with underscore", "_net", true},
		{"contains semicolon", "my_net;", true},
		{"contains control char newline", "net\n", true},
		{"contains control char tab", "net\t", true},
		{"contains space", "my net", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetworkName(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateUserName_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple user", "root", false},
		{"numeric UID", "1000", false},
		{"user and group", "root:root", false},
		{"UID and GID", "1000:1000", false},
		{"user and GID", "root:1000", false},
		{"NIS Samba compatible user", "domain-users$", false},
		{"NIS Samba user and group", "domain-users$:domain-users$", false},
		{"empty user name", "", false},

		{"too many colons", "root:root:root", true},
		{"invalid user starting character", "-root", true},
		{"contains spaces", "root name", true},
		{"contains semicolon shell char", "root;rm", true},
		{"contains newline", "root\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserName(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "input: %q", tt.input)
			} else {
				assert.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_TildeExpansion_ErrorScenario(t *testing.T) {
	t.Run("home dir error triggers path resolution failure", func(t *testing.T) {
		mfs := &exprMockFS{
			homeDirErr: errors.New("cannot fetch home directory"),
			MockFileSystem: MockFileSystem{
				WD: "/work",
			},
		}

		_, err := NewExpressionResolverWithFS(nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user home directory")
		assert.Contains(t, err.Error(), "cannot fetch home directory")
	})
}

func TestUnit_Config_RecursiveAndNestedExpressions_Scenarios(t *testing.T) {
	t.Parallel()

	t.Run("deep triple nested dynamic evaluation fallback sequence", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"VAR_A": "found_a",
				"VAR_B": "",
			},
		}

		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Triple nesting default values:
		// {{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-default}}}}}}
		// VAR_C is unset -> goes to VAR_B
		// VAR_B is empty -> goes to VAR_A
		// VAR_A is "found_a" -> resolves to "found_a"
		val := r.resolveString("{{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-default}}}}}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "found_a", val)
	})

	t.Run("deep triple nested fallback to final literal default", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"VAR_A": "",
				"VAR_B": "",
			},
		}

		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Triple nesting default values ending in final default:
		// {{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-final_fallback}}}}}}
		val := r.resolveString("{{env:VAR_C:-{{env:VAR_B:-{{env:VAR_A:-final_fallback}}}}}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "final_fallback", val)
	})
}

func TestUnit_Config_ParseMountFlag_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		expected MountConfig
		wantErr bool
	}{
		{
			name:  "all options specified",
			input: "type=bind,source=/src,target=/dst,readonly=true,optional=true",
			expected: MountConfig{
				Type:     "bind",
				Source:   ConfigPath{Raw: "/src"},
				Target:   ConfigPath{Raw: "/dst"},
				ReadOnly: true,
				Optional: true,
			},
			wantErr: false,
		},
		{
			name:  "implicit bind type",
			input: "source=/src,target=/dst,readonly,optional",
			expected: MountConfig{
				Type:     "bind",
				Source:   ConfigPath{Raw: "/src"},
				Target:   ConfigPath{Raw: "/dst"},
				ReadOnly: true,
				Optional: true,
			},
			wantErr: false,
		},
		{
			name:  "src and dst aliases",
			input: "src=/src,dst=/dst",
			expected: MountConfig{
				Type:   "bind",
				Source: ConfigPath{Raw: "/src"},
				Target: ConfigPath{Raw: "/dst"},
			},
			wantErr: false,
		},
		{
			name:  "destination alias",
			input: "source=/src,destination=/dst",
			expected: MountConfig{
				Type:   "bind",
				Source: ConfigPath{Raw: "/src"},
				Target: ConfigPath{Raw: "/dst"},
			},
			wantErr: false,
		},
		{
			name:    "invalid mount format missing equal",
			input:   "type=bind,source",
			wantErr: true,
		},
		{
			name:    "invalid readonly boolean value",
			input:   "target=/dst,readonly=not_a_bool",
			wantErr: true,
		},
		{
			name:    "invalid optional boolean value",
			input:   "target=/dst,optional=not_a_bool",
			wantErr: true,
		},
		{
			name:    "unknown mount option",
			input:   "target=/dst,unknown=value",
			wantErr: true,
		},
		{
			name:    "missing target required",
			input:   "source=/src",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ParseMountFlag(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, res)
			}
		})
	}
}

func TestUnit_Config_ParseDeviceConfig_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected DeviceConfig
		ok       bool
	}{
		{
			name:  "simple host only is mapped to both",
			input: "/dev/sda",
			expected: DeviceConfig{
				Source:      ConfigPath{Raw: "/dev/sda"},
				Destination: ConfigPath{Raw: "/dev/sda"},
				Permissions: "rwm",
			},
			ok: true,
		},
		{
			name:  "host and container mapping",
			input: "/dev/sda:/dev/sdb",
			expected: DeviceConfig{
				Source:      ConfigPath{Raw: "/dev/sda"},
				Destination: ConfigPath{Raw: "/dev/sdb"},
				Permissions: "rwm",
			},
			ok: true,
		},
		{
			name:  "host container and permissions",
			input: "/dev/sda:/dev/sdb:rm",
			expected: DeviceConfig{
				Source:      ConfigPath{Raw: "/dev/sda"},
				Destination: ConfigPath{Raw: "/dev/sdb"},
				Permissions: "rm",
			},
			ok: true,
		},
		{
			name:  "windows style drive path",
			input: "C:\\dev\\sda:/dev/sdb:wm",
			expected: DeviceConfig{
				Source:      ConfigPath{Raw: "C:\\dev\\sda"},
				Destination: ConfigPath{Raw: "/dev/sdb"},
				Permissions: "wm",
			},
			ok: true,
		},
		{
			name:  "empty string input is invalid",
			input: "",
			ok:    false,
		},
		{
			name:  "missing host path but starts with colon",
			input: ":/dev/sda",
			ok:    false,
		},
		{
			name:  "missing container path but ends with colon",
			input: "/dev/sda:",
			ok:    false,
		},
		{
			name:  "invalid permissions suffix",
			input: "/dev/sda:/dev/sdb:xyz",
			ok:    false,
		},
		{
			name:  "multiple colons in container path",
			input: "/dev/sda:/dev/sdb:/dev/sdc:rm",
			ok:    false,
		},
		{
			name:  "multiple colons with invalid suffix",
			input: "/dev/sda:/dev/sdb:/dev/sdc:xyz",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, ok := ParseDeviceConfig(tt.input)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.expected, res)
			}
		})
	}
}

func TestUnit_Config_MountAndDeviceResolve_Scenarios(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("MountConfig target parent directory traversal is rejected", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/src"},
			Target: ConfigPath{Raw: "/dst/../app"},
		}
		_, err := mc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("MountConfig target relative path is rejected", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/src"},
			Target: ConfigPath{Raw: "relative_target"},
		}
		_, err := mc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})

	t.Run("DeviceConfig destination parent directory traversal is rejected", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/sda"},
			Destination: ConfigPath{Raw: "/dev/../sda"},
		}
		_, err := dc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
	})

	t.Run("DeviceConfig destination relative path is rejected", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/sda"},
			Destination: ConfigPath{Raw: "relative_dest"},
		}
		_, err := dc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination must be an absolute path")
	})
}

func TestUnit_Config_AnchorBoundaries_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty anchor path returns error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"EMPTY_ENV": "",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Anchor resolves to an empty string. validateAnchorBoundaries should return an error.
		_, err = ResolvePath("{{env:EMPTY_ENV}}/some/file", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")
	})
}

func TestUnit_Config_ValidateWorkdir_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty workdir is valid", "", false},
		{"absolute path is valid", "/app", false},
		{"relative path is invalid", "app", true},
		{"contains parent directory", "/app/../bin", true},
		{"contains backslash", "/app\\bin", true},
		{"contains illegal character", "/app$bin", true},
		{"contains illegal asterisk", "/app*bin", true},
		{"contains illegal question", "/app?bin", true},
		{"contains illegal space", "/app bin", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkdir(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateEnvKey_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid key uppercase", "VAR_NAME", false},
		{"valid key with digits", "VAR123", false},
		{"valid key lowercase", "var_name", false},
		{"empty key is invalid", "", true},
		{"starts with digit is invalid", "123VAR", true},
		{"contains hyphen is invalid", "VAR-NAME", true},
		{"contains space is invalid", "VAR NAME", true},
		{"contains dot is invalid", "VAR.NAME", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvKey(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateImageName_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty image is valid", "", false},
		{"valid simple", "ubuntu", false},
		{"valid with registry", "docker.io/library/ubuntu", false},
		{"valid with tag", "ubuntu:latest", false},
		{"valid with digest", "ubuntu@sha256:abcdef", false},
		{"invalid starting char", "/ubuntu", true},
		{"multiple at symbols", "ubuntu@sha256:abc@def", true},
		{"invalid character", "ubuntu$latest", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateDNS_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty DNS is valid", "", false},
		{"valid IPv4", "1.1.1.1", false},
		{"valid IPv6", "2001:db8::1", false},
		{"invalid IP representation", "999.999.999.999", true},
		{"invalid hostname representation", "dns.google", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNS(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateAddHost_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty add host is valid", "", false},
		{"valid host and gateway", "myhost:host-gateway", false},
		{"valid host and IP", "myhost:1.1.1.1", false},
		{"invalid format missing IP", "myhost", true},
		{"invalid IP address", "myhost:999.999.999.999", true},
		{"invalid hostname with leading dot", ".myhost:1.1.1.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddHost(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_ValidateCapability_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty capability is valid", "", false},
		{"valid standard capability", "SYS_ADMIN", false},
		{"valid prefixed capability", "CAP_SYS_ADMIN", false},
		{"invalid lowercase capability", "sys_admin", true},
		{"invalid character hyphen", "SYS-ADMIN", true},
		{"invalid ending character", "SYS_ADMIN$", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapability(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
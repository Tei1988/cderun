package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUnit_Config_ValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Basic image", "alpine", false},
		{"Image with tag", "alpine:latest", false},
		{"Image with registry", "docker.io/library/alpine:latest", false},
		{"Image with port", "localhost:5000/my-image", false},
		{"Image with digest", "alpine@sha256:abcdef123456", false},
		{"Complex image", "my.registry.com:5000/user/repo:tag@sha256:digest", false},
		{"Image with underscore", "my_image", false},
		{"Image with dot", "my.image", false},
		{"Empty image", "", false}, // Allowed (handled in resolver)
		{"Invalid character (space)", "my image", true},
		{"Invalid character (control)", "alpine\n", true},
		{"Invalid character (semicolon)", "alpine;rm -rf /", true},
		{"Invalid character (shell)", "alpine|ls", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageName(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateEnvKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Standard key", "MY_VAR", false},
		{"Key with numbers", "VAR123", false},
		{"Key starting with underscore", "_HIDDEN", false},
		{"Key with lowercase", "myVar", false},
		{"Empty key", "", true},
		{"Key starting with number", "123VAR", true},
		{"Key with hyphen", "MY-VAR", true},
		{"Key with space", "MY VAR", true},
		{"Key with dot", "MY.VAR", true},
		{"Key with control char", "MY\nVAR", true},
		{"Key with shell char", "MY;VAR", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvKey(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidatePathChars(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Safe path", "safe/path", false},
		{"Path with space", "safe path", false},
		{"Path with tab (control)", "path/with/\t/tab", true},
		{"Path with newline (control)", "path/with/\n/newline", true},
		{"Path with carriage return (control)", "path/with/\r/return", true},
		{"Path with null byte", "path/with/\x00/null", true},
		{"Path with escape char", "path/with/\x1b/escape", true},
		{"Path with delete char", "path/with/\x7f/delete", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathChars(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ResolvePath_AnchorBoundary(t *testing.T) {
	t.Parallel()
	home := "/home/user"
	pwd := "/work"

	tests := []struct {
		name        string
		input       string
		expectedErr string
		extraEnv    map[string]string
		extraDirs   map[string]bool
		extraFiles  map[string][]byte
	}{
		{name: "Safe home path", input: "{{HOME}}/file"},
		{name: "Safe tilde path", input: "~/file"},
		{name: "Safe pwd path", input: "{{PWD}}/file"},
		{name: "Traversal within home", input: "{{HOME}}/subdir/../file"},
		{name: "Traversal escaping home", input: "{{HOME}}/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Traversal escaping tilde", input: "~/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Traversal escaping pwd", input: "{{PWD}}/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Anchor after slash", input: "/{{HOME}}/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Bypass attempt anywhere in string", input: "foo{{HOME}}/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Tilde in path (not an anchor)", input: "foo/~"},
		{name: "Tilde at start", input: "~/file"},
		{name: "Multiple anchors", input: "{{HOME}}/subdir/{{HOME}}/file"},
		{name: "False positive traversal prefix", input: "{{HOME}}/..config/file"},
		{name: "No anchor no traversal check", input: "../../etc/passwd"}, // Relative paths are resolved against baseDir, not restricted by default unless anchor is used
		{name: "Safe env anchor", input: "{{env:MY_PATH}}/file", extraEnv: map[string]string{"MY_PATH": "/work/safe"}},
		{name: "Traversal escaping env anchor", input: "{{env:MY_PATH}}/../../etc/passwd", expectedErr: "path traversal detected", extraEnv: map[string]string{"MY_PATH": "/work/safe"}},
		{name: "Safe find_dir anchor", input: "{{find_dir:.git}}/file", extraDirs: map[string]bool{"/work/.git": true}},
		{name: "Traversal escaping find_dir anchor", input: "{{find_dir:.git}}/../../etc/passwd", expectedErr: "path traversal detected", extraDirs: map[string]bool{"/work/.git": true}},

		// Nested anchor tests
		{name: "Safe nested anchor - unset (fallback to HOME)", input: "{{env:DIR:-{{HOME}}}}/file"}, // extraEnv doesn't have DIR
		{name: "Traversal escaping nested anchor - unset (fallback to HOME)", input: "{{env:DIR:-{{HOME}}}}/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Safe nested anchor - empty string (stays empty)", input: "{{env:DIR}}/file", extraEnv: map[string]string{"DIR": ""}, expectedErr: "anchor path is empty"},
		{name: "Safe nested anchor - empty value (stays empty)", input: "{{env:DIR:-}}/file", extraEnv: map[string]string{"DIR": ""}, expectedErr: "anchor path is empty"},

		{name: "Unmatched brace anchor (still validated)", input: "{{HOME}} {{/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Inner matched anchor in unmatched outer", input: "{{ PWD {{HOME}}/../../etc/passwd", expectedErr: "path traversal detected"},
		{name: "Multiple anchors - mixed types (all must be satisfied)", input: "{{HOME}}/{{PWD}}/file", expectedErr: "path traversal detected"}, // HOME is /home/user, PWD is /work. Final is /home/user/work/file. Escapes /work boundary.
		{name: "Unresolved anchor error", input: "{{unknown:directive}}/file", expectedErr: "unresolved expression in anchor"},
		{name: "Empty anchor error", input: "{{env:UNSET}}/file", expectedErr: "anchor path is empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &MockFileSystem{
				WD:      pwd,
				HomeDir: home,
				Env:     tt.extraEnv,
				Dirs:    tt.extraDirs,
				Files:   tt.extraFiles,
			}
			r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
			require.NoError(t, err)

			_, err = ResolvePath(tt.input, pwd, r)
			if tt.expectedErr != "" {
				require.Error(t, err, "input: %q", tt.input)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Standard port", "8080:80", false},
		{"Port with protocol", "80:80/tcp", false},
		{"Single port", "80", false},
		{"Empty port", "", false},
		{"IP and port", "127.0.0.1:80:80", false},
		{"IP and container port", "127.0.0.1:80", false},
		{"Invalid host port", "abc:80", true},
		{"Invalid container port", "80:abc", true},
		{"Invalid protocol", "80:80/http", true},
		{"Port with control char", "80\n", true},
		{"Too many colons", "127.0.0.1:80:80:80", true},
		{"Invalid IP", "999.999.999.999:80:80", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateDNS(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"IPv4", "8.8.8.8", false},
		{"IPv6", "2001:4860:4860::8888", false},
		{"Empty", "", false},
		{"Invalid IP", "8.8.8.256", true},
		{"Hostname", "google.com", true},
		{"Injection attempt", "8.8.8.8; rm -rf /", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNS(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateAddHost(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Standard", "myhost:127.0.0.1", false},
		{"Host gateway", "myhost:host-gateway", false},
		{"Empty", "", false},
		{"Missing colon", "myhost127.0.0.1", true},
		{"Invalid IP", "myhost:999.999.999.999", true},
		{"Invalid Hostname", "my_host:127.0.0.1", true},
		{"Injection attempt", "myhost:127.0.0.1; rm -rf /", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddHost(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateCapability(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Standard", "SYS_ADMIN", false},
		{"NET_RAW", "NET_RAW", false},
		{"Empty", "", false},
		{"Lowercase", "sys_admin", true},
		{"Injection attempt", "SYS_ADMIN; rm -rf /", true},
		{"Space", "SYS ADMIN", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapability(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateWorkdir(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Absolute", "/app", false},
		{"Root", "/", false},
		{"Empty", "", false},
		{"Relative", "app", true},
		{"Home tilde", "~/app", true},
		{"Injection attempt", "/app; rm -rf /", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkdir(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_ValidateToolName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Basic tool name", "tool", false},
		{"Tool name with hyphen", "tool-name", false},
		{"Tool name with underscore", "tool_name", false},
		{"Tool name with double dot", "tool..name", false},
		{"Empty tool name", "", true},
		{"Absolute path tool name", "/abs/path", true},
		{"Parent directory traversal", "../parent", true},
		{"Subdirectory tool name (Linux)", "subdir/tool", true},
		{"Subdirectory tool name (Windows)", "subdir\\tool", true},
		{"Control character in tool name", "tool\tname", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_Mount_AbsoluteTarget(t *testing.T) {
	t.Parallel()
	r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{WD: "/host"})
	require.NoError(t, err)

	t.Run("absolute target is accepted", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/src"},
			Target: ConfigPath{Raw: "/tgt"},
		}
		m, err := mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "/tgt", m.Target)
	})

	t.Run("relative target is rejected", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/src"},
			Target: ConfigPath{Raw: "relative/path"},
		}
		_, err := mc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})
}

func TestUnit_Config_Device_AbsoluteDestination(t *testing.T) {
	t.Parallel()
	r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{WD: "/host"})
	require.NoError(t, err)

	t.Run("absolute destination is accepted", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/sda"},
			Destination: ConfigPath{Raw: "/dev/sda"},
			Permissions: "rwm",
		}
		d, err := dc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "/dev/sda", d.PathInContainer)
	})

	t.Run("relative destination is rejected", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/sda"},
			Destination: ConfigPath{Raw: "dev/sda"},
			Permissions: "rwm",
		}
		_, err := dc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination must be an absolute path")
	})
}

func TestUnit_Config_ResolveWithFS_SecurityValidation(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/work",
	}

	tests := []struct {
		name    string
		cli     *CLIOptions
		wantErr string
	}{
		{
			name: "Invalid control char in LogFormat",
			cli: &CLIOptions{
				Image:        "alpine",
				ImageSet:     true,
				LogFormat:    "text\t",
				LogFormatSet: true,
			},
			wantErr: "security validation failed for \"log-format\"",
		},
		{
			name: "Invalid control char in Env key",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Env:      []string{"SAFE=VALUE", "UNSAFE\n=VALUE"},
			},
			wantErr: "invalid environment variable key",
		},
		{
			name: "Multiline Env value (PEM) is allowed",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Env:      []string{"CERT=-----BEGIN CERTIFICATE-----\nMIIDDTCCAfWgAwIBAgIU...\n-----END CERTIFICATE-----"},
			},
			wantErr: "", // No error expected
		},
		{
			name: "Invalid control char in Ports element",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Ports:    []string{"8080:80\r"},
			},
			wantErr: "security validation failed for ports[0]",
		},
		{
			name: "Invalid ImageName in ResolveWithFS",
			cli: &CLIOptions{
				Image:    "alpine;rm -rf /",
				ImageSet: true,
			},
			wantErr: "security validation failed for \"image\"",
		},
		{
			name: "Invalid EnvKey in ResolveWithFS (CLI)",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Env:      []string{"INVALID-KEY=VALUE"},
			},
			wantErr: "invalid environment variable key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ResolveWithFS("tool", tt.cli, nil, nil, fs)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

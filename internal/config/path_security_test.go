package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

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
	mfs := &MockFileSystem{
		WD:      pwd,
		HomeDir: home,
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	tests := []struct {
		name    string
		input   string
		wantErr bool
		home    string
		pwd     string
	}{
		{"Safe home path", "{{HOME}}/file", false, home, pwd},
		{"Safe tilde path", "~/file", false, home, pwd},
		{"Safe pwd path", "{{PWD}}/file", false, home, pwd},
		{"Traversal within home", "{{HOME}}/subdir/../file", false, home, pwd},
		{"Traversal escaping home", "{{HOME}}/../../etc/passwd", true, home, pwd},
		{"Traversal escaping tilde", "~/../../etc/passwd", true, home, pwd},
		{"Traversal escaping pwd", "{{PWD}}/../../etc/passwd", true, home, pwd},
		{"Anchor after slash", "/{{HOME}}/../../etc/passwd", true, home, pwd},
		{"False positive traversal prefix", "{{HOME}}/..config/file", false, home, pwd},
		{"No anchor no traversal check", "../../etc/passwd", false, home, pwd}, // Relative paths are resolved against baseDir, not restricted by default unless anchor is used
		{"Multiple anchors safe", "{{HOME}}{{PWD}}/file", false, "/", "/work"},
		{"Multiple anchors unsafe second", "{{HOME}}/{{PWD}}/../../etc/passwd", true, home, pwd},
		{"Multiple anchors bypass first", "{{HOME}}{{PWD}}/../etc/passwd", true, "/", "/work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs.HomeDir = tt.home
			mfs.WD = tt.pwd
			r.Home = tt.home
			r.Pwd = tt.pwd
			_, err := ResolvePath(tt.input, tt.pwd, r)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
				assert.Contains(t, err.Error(), "path traversal detected")
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
	r := &ExpressionResolver{Pwd: "/host"}

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
	r := &ExpressionResolver{Pwd: "/host"}

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
			wantErr: "security validation failed for env[1] (key)",
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
			name: "Space in Ports element",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				Ports:    []string{"8080 :80"},
			},
			wantErr: "security validation failed for ports[0]",
		},
		{
			name: "Space in DNS element",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				DNS:      []string{"8.8.8.8 "},
			},
			wantErr: "security validation failed for dns[0]",
		},
		{
			name: "Invalid char in AddHosts",
			cli: &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
				AddHosts: []string{"host:1.2.3.4;"},
			},
			wantErr: "security validation failed for add-hosts[0]",
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

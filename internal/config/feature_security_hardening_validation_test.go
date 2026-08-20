package config

import (
	"bytes"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestLogger(t *testing.T, level string) *bytes.Buffer {
	t.Helper()
	g := logging.GetGlobalLogger()
	origWriter := g.GetWriter()
	origLevel := g.GetLevel().LowerString()
	origFormat := g.GetFormat()
	origTimestamp := g.GetTimestamp()

	buf := &bytes.Buffer{}
	logging.SetOutput(buf)
	err := logging.Init(level, "text", false)
	require.NoError(t, err)

	t.Cleanup(func() {
		logging.SetOutput(origWriter)
		err := logging.Init(origLevel, origFormat, origTimestamp)
		require.NoError(t, err)
	})

	return buf
}

func TestSecurityHardening_ValidationEnhancements(t *testing.T) {
	t.Run("sensitive mount path /root warning", func(t *testing.T) {
		buf := setupTestLogger(t, "warn")

		mfs := &MockFileSystem{}
		r := &resolver{
			fs:  mfs,
			cli: &CLIOptions{},
			res: &ResolvedConfig{
				Image:   "alpine",
				Runtime: "docker",
				Mounts: []container.Mount{
					{Type: "bind", Source: "/root/secret", Target: "/app"},
				},
			},
		}

		err := r.validateSecurity()
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Mounting highly sensitive host path \"/root/secret\" into the container reduces host security isolation")
	})

	t.Run("relaxed security options warning", func(t *testing.T) {
		buf := setupTestLogger(t, "warn")

		mfs := &MockFileSystem{}
		r := &resolver{
			fs:  mfs,
			cli: &CLIOptions{},
			res: &ResolvedConfig{
				Image:       "alpine",
				Runtime:     "docker",
				SecurityOpt: []string{"seccomp=unconfined", "apparmor=unconfined", "label=disable", "systempaths=unconfined", "no-new-privileges=false"},
			},
		}

		err := r.validateSecurity()
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "seccomp=unconfined")
		assert.Contains(t, out, "apparmor=unconfined")
		assert.Contains(t, out, "label=disable")
		assert.Contains(t, out, "systempaths=unconfined")
		assert.Contains(t, out, "no-new-privileges=false")
	})

	t.Run("sensitive disk device warnings", func(t *testing.T) {
		buf := setupTestLogger(t, "warn")

		mfs := &MockFileSystem{}
		r := &resolver{
			fs:  mfs,
			cli: &CLIOptions{},
			res: &ResolvedConfig{
				Image:   "alpine",
				Runtime: "docker",
				Devices: []container.DeviceMapping{
					{PathOnHost: "/dev/hda1", PathInContainer: "/dev/xda", CgroupPermissions: "rwm"},
					{PathOnHost: "/dev/xvda", PathInContainer: "/dev/xdb", CgroupPermissions: "rwm"},
					{PathOnHost: "/dev/mmcblk0", PathInContainer: "/dev/xdc", CgroupPermissions: "rwm"},
					{PathOnHost: "/dev/sg0", PathInContainer: "/dev/xdd", CgroupPermissions: "rwm"},
				},
			},
		}

		err := r.validateSecurity()
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "/dev/hda1")
		assert.Contains(t, out, "/dev/xvda")
		assert.Contains(t, out, "/dev/mmcblk0")
		assert.Contains(t, out, "/dev/sg0")
	})

	t.Run("env value control character rejection", func(t *testing.T) {
		mfs := &MockFileSystem{}
		r := &resolver{
			fs:  mfs,
			cli: &CLIOptions{},
			res: &ResolvedConfig{
				Image:   "alpine",
				Runtime: "docker",
				Env:     []string{"GOOD_VAR=ok", "BAD_VAR=hello\x07world"},
			},
		}

		err := r.validateSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "env[1]")
		assert.Contains(t, err.Error(), "invalid control character")
	})

	t.Run("sysctl key dot format validation", func(t *testing.T) {
		require.Error(t, ValidateSysctlKey(".net.ipv4.ip_forward"))
		require.Error(t, ValidateSysctlKey("net.ipv4.ip_forward."))
		require.Error(t, ValidateSysctlKey("net..ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
	})
}

package command

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

func TestUnit_FormatDryRunSimple(t *testing.T) {
	cfg := &container.ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello world"},
		TTY:         true,
		Interactive: true,
		Network:     "bridge",
		Remove:      true,
		ReadOnly:    true,
		Init:        true,
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host/dir", Target: "/container/dir", ReadOnly: true},
		},
		Env:         []string{"KEY1=VAL1", "BAREKEY"},
		Workdir:     "/app",
		User:        "1000:1000",
		Ports:       []string{"8080:80"},
		PublishAll:  true,
		Expose:      []string{"8080/tcp"},
		Hostname:    "myhost",
		DNS:         []string{"8.8.8.8"},
		AddHosts:    []string{"customhost:127.0.0.1"},
		Privileged:  true,
		Pid:         "host",
		ShmSize:     "256m",
		IPC:         "host",
		SecurityOpt: []string{"no-new-privileges:true"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2"},
		GPUs:        "all",
		Cgroupns:    "private",
		PidsLimit:   100,
		CPUShares:   1024,
		CpusetCpus:  "0-1",
		CpusetMems:  "0",
		Restart:     "always",
		CapAdd:      []string{"CAP_SYS_ADMIN"},
		CapDrop:     []string{"CAP_NET_RAW"},
		GroupAdd:    []string{"1001"},
		Ulimits: []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
		Sysctls: map[string]string{
			"net.ipv4.ip_forward": "1",
		},
		Devices: []container.DeviceMapping{
			{PathOnHost: "/dev/kvm", PathInContainer: "/dev/kvm", CgroupPermissions: "rwm"},
			{PathOnHost: "/dev/net/tun", PathInContainer: "/dev/tun", CgroupPermissions: "r"},
		},
		Memory:     1024 * 1024 * 512, // 512 MiB
		CPUs:       2.5,
		Entrypoint: []string{"/bin/sh", "-c"},
	}

	var buf bytes.Buffer
	formatDryRunSimple(&buf, cfg)
	output := buf.String()

	assert.Contains(t, output, `Image: alpine:latest`)
	assert.Contains(t, output, `Command: "echo" "hello world"`)
	assert.Contains(t, output, `TTY: true`)
	assert.Contains(t, output, `Interactive: true`)
	assert.Contains(t, output, `Network: bridge`)
	assert.Contains(t, output, `Remove: true`)
	assert.Contains(t, output, `ReadOnly: true`)
	assert.Contains(t, output, `Init: true`)
	assert.Contains(t, output, `Mounts: type=bind,source="/host/dir",target="/container/dir",readonly=true`)
	assert.Contains(t, output, `Env: "KEY1"="VAL1", "BAREKEY"`)
	assert.Contains(t, output, `Workdir: /app`)
	assert.Contains(t, output, `User: 1000:1000`)
	assert.Contains(t, output, `Ports: 8080:80`)
	assert.Contains(t, output, `PublishAll: true`)
	assert.Contains(t, output, `Expose: 8080/tcp`)
	assert.Contains(t, output, `Hostname: myhost`)
	assert.Contains(t, output, `DNS: 8.8.8.8`)
	assert.Contains(t, output, `AddHosts: customhost:127.0.0.1`)
	assert.Contains(t, output, `Privileged: true`)
	assert.Contains(t, output, `Pid: host`)
	assert.Contains(t, output, `ShmSize: 256m`)
	assert.Contains(t, output, `IPC: host`)
	assert.Contains(t, output, `SecurityOpt: no-new-privileges:true`)
	assert.Contains(t, output, `DNSSearch: example.com`)
	assert.Contains(t, output, `DNSOptions: ndots:2`)
	assert.Contains(t, output, `GPUs: all`)
	assert.Contains(t, output, `Cgroupns: private`)
	assert.Contains(t, output, `PidsLimit: 100`)
	assert.Contains(t, output, `CPUShares: 1024`)
	assert.Contains(t, output, `CpusetCpus: 0-1`)
	assert.Contains(t, output, `CpusetMems: 0`)
	assert.Contains(t, output, `Restart: always`)
	assert.Contains(t, output, `CapAdd: CAP_SYS_ADMIN`)
	assert.Contains(t, output, `CapDrop: CAP_NET_RAW`)
	assert.Contains(t, output, `GroupAdd: 1001`)
	assert.Contains(t, output, `Ulimits: nofile=1024:2048`)
	assert.Contains(t, output, `Sysctls: net.ipv4.ip_forward=1`)
	assert.Contains(t, output, `Devices: /dev/kvm, /dev/net/tun:/dev/tun:r`)
	assert.Contains(t, output, `Memory: 512MiB`)
	assert.Contains(t, output, `CPUs: 2.5`)
	assert.Contains(t, output, `Entrypoint: "/bin/sh" "-c"`)
}

func TestUnit_InitEarlyLogger_Validation(t *testing.T) {
	t.Run("DefaultInitialization", func(t *testing.T) {
		mfs := &config.MockFileSystem{}
		opts := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		cmd := &cobra.Command{Use: "cderun"}
		registerFlags(cmd, opts)

		err := opts.initEarlyLogger(cmd)
		require.NoError(t, err)
	})

	t.Run("InvalidLogLevel", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_LEVEL": "invalid_level"},
		}
		opts := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		cmd := &cobra.Command{Use: "cderun"}
		registerFlags(cmd, opts)

		err := opts.initEarlyLogger(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log level")
	})

	t.Run("InvalidLogFormat", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_FORMAT": "xml"},
		}
		opts := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		cmd := &cobra.Command{Use: "cderun"}
		registerFlags(cmd, opts)

		err := opts.initEarlyLogger(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log format")
	})

	t.Run("InvalidLogTimestamp", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_TIMESTAMP": "not_a_bool"},
		}
		opts := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		cmd := &cobra.Command{Use: "cderun"}
		registerFlags(cmd, opts)

		err := opts.initEarlyLogger(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid boolean value for log-timestamp")
	})
}

package runtime

import (
	"testing"

	"cderun/internal/container"
)

func BenchmarkCLIArgBuilder_BuildAndVerify(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		builder := NewCLIArgBuilder()
		builder.AddJoinedFlag("--workdir", "/app", true)
		builder.AddJoinedFlag("--user", "1000:1000", true)
		builder.AddJoinedFlag("--net", "bridge", true)
		builder.AddBoolFlag("--tty", true)
		builder.AddBoolFlag("--interactive", true)
		builder.AddBoolFlag("--read-only", true)
		builder.AddRepeatJoinedFlag("--env", []string{"FOO=BAR", "BAZ=QUX"})
		builder.AddRepeatJoinedFlag("--cap-add", []string{"SYS_PTRACE", "NET_ADMIN"})
		builder.AddPositionals("alpine:latest", "echo", "hello")

		argv := builder.Build()
		if err := builder.VerifyStructure(argv, -1); err != nil {
			b.Fatalf("VerifyStructure failed: %v", err)
		}
	}
}

func BenchmarkNerdctlRuntime_ValidateAndBuildArgs(b *testing.B) {
	r, err := NewNerdctlRuntime("/run/containerd/containerd.sock")
	if err != nil {
		b.Fatalf("NewNerdctlRuntime failed: %v", err)
	}

	cfg := &container.ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Workdir:     "/app",
		User:        "1000:1000",
		Network:     "bridge",
		TTY:         true,
		Interactive: true,
		ReadOnly:    true,
		Env:         []string{"FOO=BAR", "BAZ=QUX", "APP_ENV=production"},
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host/path", Target: "/container/path", ReadOnly: true},
		},
		CapAdd:   []string{"SYS_PTRACE", "NET_ADMIN"},
		CapDrop:  []string{"ALL"},
		Sysctls:  map[string]string{"net.ipv4.ip_forward": "1"},
		Labels:   map[string]string{"app": "cderun", "env": "prod"},
		Ulimits:  []container.Ulimit{{Name: "nofile", Soft: 65535, Hard: 65535}},
		Devices:  []container.DeviceMapping{{PathOnHost: "/dev/kvm", PathInContainer: "/dev/kvm", CgroupPermissions: "rwm"}},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := r.ValidateConfig(cfg); err != nil {
			b.Fatalf("ValidateConfig failed: %v", err)
		}
	}
}

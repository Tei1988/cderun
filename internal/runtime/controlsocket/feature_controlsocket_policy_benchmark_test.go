package controlsocket

import (
	"testing"

	"cderun/internal/container"
)

func BenchmarkValidateInheritedCeiling(b *testing.B) {
	b.Run("StandardChildMatchesParent", func(b *testing.B) {
		child := &container.ContainerConfig{
			Network:     "bridge",
			Pid:         "",
			IPC:         "",
			Cgroupns:    "",
			OciRuntime:  "runc",
			SecurityOpt: []string{"no-new-privileges:true"},
		}
		parent := &container.ContainerConfig{
			Network:     "bridge",
			Pid:         "",
			IPC:         "",
			Cgroupns:    "",
			OciRuntime:  "runc",
			SecurityOpt: []string{"no-new-privileges:true"},
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ValidateInheritedCeiling(child, parent); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("CapabilitiesValidation", func(b *testing.B) {
		child := &container.ContainerConfig{
			CapAdd:      []string{"NET_ADMIN", "SYS_PTRACE"},
			CapDrop:     []string{"MKNOD"},
			SecurityOpt: []string{"no-new-privileges"},
		}
		parent := &container.ContainerConfig{
			CapAdd:      []string{"CAP_NET_ADMIN", "CAP_SYS_PTRACE", "CAP_CHOWN"},
			CapDrop:     []string{"CAP_MKNOD"},
			SecurityOpt: []string{"no-new-privileges"},
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ValidateInheritedCeiling(child, parent); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("HardeningSecurityOptionsOnly", func(b *testing.B) {
		child := &container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges:true"},
		}
		parent := &container.ContainerConfig{
			SecurityOpt: []string{"seccomp=unconfined"},
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ValidateInheritedCeiling(child, parent); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("ParentPrivilegedShortCircuit", func(b *testing.B) {
		child := &container.ContainerConfig{
			Privileged:  false,
			CapAdd:      []string{"SYS_ADMIN"},
			SecurityOpt: []string{"no-new-privileges"},
		}
		parent := &container.ContainerConfig{
			Privileged: true,
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ValidateInheritedCeiling(child, parent); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}

package config

import "testing"

// overlayUpperDir mimics the OverlayFS upperdir discovered from /proc/self/mountinfo
// inside a container, which is registered as the fallback root mapping (target "/").
const overlayUpperDir = "/var/lib/docker/containerd/daemon/io.containerd.snapshotter.v1.overlayfs/snapshots/17207/fs"

const (
	baseHostHome = "/Users/dev"
	baseHostPwd  = "/Users/dev/work"
)

// newNestedResolver builds a resolver that emulates a Level 1 container whose Base Host
// is a macOS machine: the execution host home (/home/tool) differs from the Base Host
// home (/Users/dev), and an OverlayFS fallback root mapping is present.
func newNestedResolver(t *testing.T, containerHome string, extraMounts ...MountMapping) *ExpressionResolver {
	t.Helper()

	fs := &MockFileSystem{
		WD:      baseHostPwd,
		HomeDir: containerHome,
		Env:     map[string]string{},
		Dirs:    map[string]bool{"/": true},
	}

	hostCtx := &HostContext{
		Level:      1,
		HomeDir:    baseHostHome,
		WorkingDir: baseHostPwd,
		Mounts: append(extraMounts,
			MountMapping{Source: baseHostPwd, Target: baseHostPwd, Level: 1},
			MountMapping{Source: baseHostHome + "/.certs/all.pem", Target: "/etc/ssl/certs/ca-certificates.crt", Level: 1},
			MountMapping{Source: overlayUpperDir, Target: "/", Level: 1},
		),
	}

	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	return r
}

// TestUnit_ReverseResolution_BaseHostPathBypassesOverlayFallback verifies that paths which
// already live in the Base Host namespace are not rewritten through the OverlayFS fallback
// root mapping. Rewriting them produced mount sources such as
// "<upperdir>/Users/dev/.certs/all.pem", which do not exist on the Base Host and made the
// container runtime fail with "bind source path does not exist".
func TestUnit_ReverseResolution_BaseHostPathBypassesOverlayFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "BASE_HOME anchored file outside every mount target",
			path: "{{BASE_HOME}}/.certs/all.pem",
			want: baseHostHome + "/.certs/all.pem",
		},
		{
			name: "BASE_HOME anchored directory outside every mount target",
			path: "{{BASE_HOME}}/.kiro",
			want: baseHostHome + "/.kiro",
		},
		{
			name: "BASE_HOME itself",
			path: "{{BASE_HOME}}",
			want: baseHostHome,
		},
		{
			name: "BASE_PWD anchored path",
			path: "{{BASE_PWD}}/src",
			want: baseHostPwd + "/src",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolvePath(tt.path, "", newNestedResolver(t, "/home/tool"))
			if err != nil {
				t.Fatalf("ResolvePath(%q) returned error: %v", tt.path, err)
			}
			if got != tt.want {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestUnit_ReverseResolution_ContainerPathStillUsesOverlayFallback guards the behaviour the
// fallback root mapping exists for: container scratch-space paths must keep being rewritten
// to their Base Host location.
func TestUnit_ReverseResolution_ContainerPathStillUsesOverlayFallback(t *testing.T) {
	t.Parallel()

	r := newNestedResolver(t, "/home/tool")

	got, err := ResolvePath("/tmp/cderun-snap-abc", "", r)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if want := overlayUpperDir + "/tmp/cderun-snap-abc"; got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
}

// TestUnit_ReverseResolution_ExplicitMountTargetWinsOverBaseHostRoot ensures the guard only
// applies to the fallback root mapping: a path covered by a real mount target is still
// translated through that mount, even when it also sits under a Base Host root.
func TestUnit_ReverseResolution_ExplicitMountTargetWinsOverBaseHostRoot(t *testing.T) {
	t.Parallel()

	// The container home is nested inside the Base Host home, and it is bind mounted
	// from a different Base Host location.
	r := newNestedResolver(t, baseHostHome+"/container-home",
		MountMapping{Source: "/Users/dev/.kiro", Target: baseHostHome + "/container-home/.kiro", Level: 1},
	)

	got, err := ResolvePath(baseHostHome+"/container-home/.kiro/settings.json", "", r)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if want := "/Users/dev/.kiro/settings.json"; got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
}

// TestUnit_ReverseResolution_AmbiguousBaseHostRootKeepsFallback documents that the guard is
// skipped when a Base Host root is identical to its execution host counterpart. In that case
// the direction of the mapping cannot be inferred from the path alone, so the mount table
// lookup (including the fallback root mapping) stays in charge.
func TestUnit_ReverseResolution_AmbiguousBaseHostRootKeepsFallback(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{
		WD:      "/root",
		HomeDir: "/root",
		Env:     map[string]string{},
		Dirs:    map[string]bool{"/": true},
	}
	hostCtx := &HostContext{
		Level:      1,
		HomeDir:    "/root", // identical to the container home
		WorkingDir: "/root",
		Mounts: []MountMapping{
			{Source: overlayUpperDir, Target: "/", Level: 1},
		},
	}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	got, err := ResolvePath("{{BASE_HOME}}/.cache", "", r)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if want := overlayUpperDir + "/root/.cache"; got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
}

// TestUnit_ReverseResolution_BaseHostLevelZeroIsUntouched verifies that nothing changes when
// cderun runs directly on the Base Host.
func TestUnit_ReverseResolution_BaseHostLevelZeroIsUntouched(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{
		WD:      baseHostPwd,
		HomeDir: baseHostHome,
		Env:     map[string]string{},
		Dirs:    map[string]bool{"/": true},
	}
	r, err := NewExpressionResolverWithFS(&HostContext{Level: 0}, fs)
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	got, err := ResolvePath("{{BASE_HOME}}/.certs/all.pem", "", r)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if want := baseHostHome + "/.certs/all.pem"; got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
}

// TestUnit_ReverseResolution_OverlappingBaseHostRootKeepsFallback covers a Base Host root
// that overlaps a directory which also exists inside the container: an Ubuntu host user
// (/home/ubuntu) running a container as root (/root) whose image ships its own
// /home/ubuntu. Containment in the Base Host home is not enough to conclude the path came
// from {{BASE_HOME}}, so a path that is physically present on the Execution Host must keep
// using the OverlayFS fallback mapping.
func TestUnit_ReverseResolution_OverlappingBaseHostRootKeepsFallback(t *testing.T) {
	t.Parallel()

	fs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/root",
		Env:     map[string]string{},
		Dirs: map[string]bool{
			"/":                  true,
			"/home/ubuntu":       true,
			"/home/ubuntu/cache": true,
		},
	}
	hostCtx := &HostContext{
		Level:      1,
		HomeDir:    "/home/ubuntu", // Base Host home, distinct from the container home
		WorkingDir: "/home/ubuntu/project",
		Mounts: []MountMapping{
			{Source: overlayUpperDir, Target: "/", Level: 1},
		},
	}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	// Present inside the container: a genuine scratch-space path, still rewritten.
	got, err := ResolvePath("/home/ubuntu/cache", "", r)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if want := overlayUpperDir + "/home/ubuntu/cache"; got != want {
		t.Errorf("container-local path: ResolvePath = %q, want %q", got, want)
	}

	// Absent inside the container: cannot be scratch space, so it is a Base Host path.
	got, err = ResolvePath("{{BASE_HOME}}/.certs/all.pem", "", r)
	if err != nil {
		t.Fatalf("ResolvePath returned error: %v", err)
	}
	if want := "/home/ubuntu/.certs/all.pem"; got != want {
		t.Errorf("base host path: ResolvePath = %q, want %q", got, want)
	}
}

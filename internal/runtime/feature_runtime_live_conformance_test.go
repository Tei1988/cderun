//go:build runtime

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLiveConformance_RuntimeAdapter(t *testing.T) {
	engine := os.Getenv("CDERUN_ENGINE")
	if engine == "" {
		t.Skip("CDERUN_ENGINE environment variable not set; skipping live runtime conformance test")
	}

	socketPath := os.Getenv("CDERUN_SOCKET_PATH")
	if socketPath == "" {
		switch engine {
		case "containerd":
			socketPath = "/run/containerd/containerd.sock"
		case "docker":
			socketPath = "/var/run/docker.sock"
		case "podman":
			if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
				socketPath = filepath.Join(xdg, "podman", "podman.sock")
			} else if os.Geteuid() == 0 {
				socketPath = "/run/podman/podman.sock"
			} else {
				socketPath = fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid())
			}
		}
	}

	factory := func(t *testing.T) ContainerRuntime {
		var rt ContainerRuntime
		var err error
		switch engine {
		case "containerd":
			rt, err = NewContainerdRuntime(socketPath)
		case "docker":
			rt, err = NewDockerRuntime(socketPath)
		case "podman":
			rt, err = NewPodmanRuntime(socketPath)
		default:
			t.Fatalf("Unsupported engine %q for live conformance testing", engine)
		}
		if err != nil {
			t.Fatalf("Failed to initialize runtime %q on socket %q: %v", engine, socketPath, err)
		}
		return rt
	}

	// Pre-flight check: initialize once and ensure image is pulled
	initRt := factory(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pullErr := initRt.PullImage(ctx, "docker.io/library/alpine:latest", "missing", 3, time.Second)
	skipIfRuntimeBroken(t, pullErr)
	require.NoError(t, pullErr, "Pre-flight PullImage for docker.io/library/alpine:latest must succeed")
	initRt.Close()

	var caps ConformanceCapabilities
	switch engine {
	case "containerd":
		caps = ConformanceCapabilities{
			SupportsVolumes:    false,
			SupportsTmpfs:      true,
			SupportsPorts:      false,
			SupportsGPUs:       false,
			SupportsDNSSearch:  false,
			SupportsDNSOptions: false,
			SupportsInit:       false,
			RequiresCapPrefix:  true,
		}
	case "docker", "podman":
		caps = ConformanceCapabilities{
			SupportsVolumes:    true,
			SupportsTmpfs:      true,
			SupportsPorts:      true,
			SupportsGPUs:       true,
			SupportsDNSSearch:  true,
			SupportsDNSOptions: true,
			SupportsInit:       true,
			RequiresCapPrefix:  false,
		}
	}

	RunConformanceTests(t, factory, caps)
}

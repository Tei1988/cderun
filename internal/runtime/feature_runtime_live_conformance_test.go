//go:build runtime

package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func defaultSocketPathForLiveEngine(engine string) string {
	switch engine {
	case "containerd":
		return "/run/containerd/containerd.sock"
	case "podman":
		uid := os.Getuid()
		return fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
	case "docker":
		return "/var/run/docker.sock"
	default:
		return ""
	}
}

func TestConformance_LiveSocketRuntime(t *testing.T) {
	engineName := os.Getenv("CDERUN_ENGINE")
	if engineName == "" {
		engineName = "containerd"
	}

	socketPath := os.Getenv("CDERUN_SOCKET_PATH")
	if socketPath == "" {
		socketPath = defaultSocketPathForLiveEngine(engineName)
	}

	if socketPath == "" {
		t.Fatalf("-tags=runtime specified but no socket path could be determined for engine %q", engineName)
	}

	conn, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		t.Fatalf("Live runtime socket check failed for %s (%s): %v. Live tests must fail when socket is unreachable.", engineName, socketPath, err)
	}
	_ = conn.Close()

	var caps ConformanceCapabilities
	var factory func(t *testing.T) ContainerRuntime

	switch engineName {
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
		factory = func(t *testing.T) ContainerRuntime {
			rt, err := NewContainerdRuntime(socketPath)
			require.NoError(t, err)
			return rt
		}
	case "docker":
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
		factory = func(t *testing.T) ContainerRuntime {
			rt, err := NewDockerRuntime(socketPath)
			require.NoError(t, err)
			return rt
		}
	case "podman":
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
		factory = func(t *testing.T) ContainerRuntime {
			rt, err := NewPodmanRuntime(socketPath)
			require.NoError(t, err)
			return rt
		}
	default:
		t.Fatalf("Unsupported live runtime engine %q", engineName)
	}

	// Ensure docker.io/library/alpine:latest is pulled and available prior to running conformance suite
	setupRt := factory(t)
	pullCtx, pullCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer pullCancel()
	err = setupRt.PullImage(pullCtx, "docker.io/library/alpine:latest", "missing", 3, 1*time.Second)
	closeErr := setupRt.Close()
	require.NoError(t, err, "Failed to pull docker.io/library/alpine:latest image for live conformance test")
	require.NoError(t, closeErr, "Failed to close setup runtime instance")

	RunConformanceTests(t, factory, caps)
}

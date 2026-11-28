//go:build runtime

package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// defaultSocketPathForLiveEngine returns the default UNIX socket path for a given container engine.
func defaultSocketPathForLiveEngine(engine string) string {
	switch engine {
	case "containerd":
		return "/run/containerd/containerd.sock"
	case "podman":
		userSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid())
		if _, err := os.Stat(userSock); err == nil {
			return userSock
		}
		return "/run/podman/podman.sock"
	case "docker":
		return "/var/run/docker.sock"
	default:
		return ""
	}
}

// discoverEngineAndSocket discovers the container engine and socket path based on environment variables
// and filesystem socket availability, adhering to cderun's engine auto-detection precedence order.
func discoverEngineAndSocket() (string, string) {
	engine := os.Getenv("CDERUN_ENGINE")
	socket := os.Getenv("CDERUN_SOCKET_PATH")

	if engine != "" && socket != "" {
		return engine, socket
	}

	if engine != "" && socket == "" {
		return engine, defaultSocketPathForLiveEngine(engine)
	}

	if engine == "" && socket != "" {
		// Deduce engine from socket path basename
		base := strings.TrimRight(socket, "/")
		if idx := strings.LastIndexByte(base, '/'); idx != -1 {
			base = base[idx+1:]
		}
		if strings.Contains(base, "podman") {
			return "podman", socket
		}
		if strings.Contains(base, "containerd") {
			return "containerd", socket
		}
		return "docker", socket
	}

	// Engine and socket are both unspecified: auto-detect from filesystem in standard priority order:
	// Docker -> containerd -> Podman, defaulting to Docker if none are detected.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "docker", "/var/run/docker.sock"
	}
	if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
		return "containerd", "/run/containerd/containerd.sock"
	}
	podmanUserSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid())
	if _, err := os.Stat(podmanUserSock); err == nil {
		return "podman", podmanUserSock
	}
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "podman", "/run/podman/podman.sock"
	}

	return "docker", "/var/run/docker.sock"
}

// TestConformance_LiveSocketRuntime runs the ContainerRuntime conformance test suite
// against a live container engine daemon connected via a UNIX domain socket.
// It detects the target engine and socket path via CDERUN_ENGINE and CDERUN_SOCKET_PATH
// or automatically discovers them using standard filesystem probing.
// When the target socket is unreachable, the test fails immediately.
func TestConformance_LiveSocketRuntime(t *testing.T) {
	engineName, socketPath := discoverEngineAndSocket()

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

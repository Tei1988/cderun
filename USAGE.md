# Advanced Usage: Nested Execution on macOS

This guide provides setup instructions for running **Nested Execution** (running `cderun` recursively inside a container managed by `cderun`) on macOS environments.

---

## The Challenge on macOS

Nested Execution relies on mounting the `cderun` binary and the container runtime socket (`docker.sock` or `podman.sock`) from the host into the container.

On macOS, this introduces two specific challenges:

1. **OS/Architecture Mismatch**: The host `cderun` binary on macOS is compiled for Darwin (`GOOS=darwin`). However, the container runs Linux. A macOS binary cannot be executed inside a Linux container.
2. **Socket GIDs**: The host's `stat` on `/var/run/docker.sock` might report a different Group ID (GID) than what is present inside the VM/container context, preventing automatic GID assignment from granting appropriate permissions.

To run Nested Execution successfully on macOS, you must configure a Linux-compatible `cderun` binary and ensure the container user has permission to access the container runtime socket.

---

## Step 1: Specifying the Linux Binary

The inner `cderun` executing inside the container must be a Linux binary matching the target container's architecture (e.g., `linux/arm64` for Apple Silicon, or `linux/amd64` for Intel).

### 1. Compile the Linux Binary

You can cross-compile the Linux binary on your macOS host:

```bash
# For Apple Silicon Macs (M1/M2/M3/M4)
GOOS=linux GOARCH=arm64 go build -o cderun_linux_arm64 main.go

# For Intel-based Macs
GOOS=linux GOARCH=amd64 go build -o cderun_linux_amd64 main.go
```

### 2. Configure the Binary Path

Specify the pre-built Linux binary path using `--mount-cderun-path` on the CLI or in your global configuration file (`.cderun.yaml`):

```yaml
# .cderun.yaml
defaults:
  mountCderun: true
  mountCderunPath: "./cderun_linux_arm64"
```

---

## Step 2: Socket GID Mapping (`groupAdd`)

To execute container commands recursively, the inner `cderun` needs access to the mapped socket file (e.g. `/var/run/docker.sock`). If you are running the container as a non-root user, you will face `permission denied` errors unless the container user belongs to the socket's owner group.

On macOS, since the socket is managed by a lightweight Linux VM (such as Docker Desktop or Podman), the automatic GID lookup on the host may not resolve correctly or match the VM's permissions.

To grant the necessary permissions, add the appropriate GID (e.g. `102` for Docker Desktop on Mac) explicitly to the supplementary groups via the `groupAdd` configuration or `--cderun-group-add` CLI flag.

```yaml
# .cderun.yaml
defaults:
  groupAdd:
    - "102"
```

---

## Complete `.cderun.yaml` Example

To achieve seamless nested execution on macOS, create a global configuration file (e.g., `.cderun.yaml` in your project root or `~/.config/cderun/.cderun.yaml`):

```yaml
runtime: docker
defaults:
  mountCderun: true
  mountCderunPath: "./cderun_linux_arm64"
  mountSocket: true
  groupAdd:
    - "102"
```

Once configured, any recursive invocation of `cderun` (such as mounting tools defined in `.tools.yaml`) will run reliably inside your ephemeral containers on macOS.

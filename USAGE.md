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

To grant the necessary permissions, you must add the correct socket GID explicitly to the supplementary groups via the `groupAdd` configuration or the `--cderun-group-add` CLI flag.

> **Important**: The value `102` shown below is merely an **example**. The actual GID depends on your specific container runtime installation and environment on your machine. You must determine and configure the correct GID for your system.

### 1. Determine the Socket GID on Your System

To find the numeric GID of the socket as seen inside the container environment, run `cderun` with root user privileges to list the socket's file details:

```bash
cderun --mount-socket --user root alpine ls -ln /var/run/docker.sock
```

The output will look similar to this:

```text
srw-rw---- 1 0 102 0 Jun 15 10:00 /var/run/docker.sock
```

In this output, the third column (`0`) is the owner UID (root), and the fourth column (`102`) is the owner GID. This GID (`102` in this example) is the value you must use.

### 2. Configure the GID (Numeric Only)

You must ensure that the value passed to `groupAdd` or `--cderun-group-add` is **numeric** (e.g. `"102"`), not a group name (e.g. `"docker"`). Numeric values are required because container runtime adapters (such as `containerd`) resolve permissions directly inside the container using GIDs and do not perform group name lookups.

Add the determined numeric GID to your configuration:

```yaml
# .cderun.yaml
defaults:
  groupAdd:
    - "102" # Replace with your actual socket GID determined in Step 2.1
```

Or pass it as a CLI option when running the command:

```bash
cderun node app.js --cderun-group-add 102
```

### 3. Verify Socket Access

To verify that socket access has been successfully configured, execute a command as a non-root user inside a container and verify that `cderun` can communicate with the runtime socket without permission errors:

```bash
cderun --mount-socket --user 1000 alpine sh -c "cderun --diagnosis"
```

If the configuration is correct, the nested diagnosis will complete successfully and display the container runtime status instead of returning a `permission denied` error.

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

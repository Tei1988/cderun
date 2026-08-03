# Feature Specification: Container Command Execution

## Overview

`cderun` runs execution commands within ephemeral containers, providing clean, isolated, and reproducible workspace environments without requiring tools (such as Node.js or Python) to be installed locally on the host machine.

Command-line parameters are parsed according to the rules defined in the [Argument Parsing Feature Specification](./argument-parsing.md) to determine the exact command structure passed to the container.

## Core Features

### Command Wrapping

- Executes arbitrary commands seamlessly inside containers.
- Automatically removes the container after execution (`--rm`).
- Preserves and returns the exact process exit codes and standard output streams.

### Interactive Terminal Support

- Allocates pseudo-TTYs for interactive command flows (`--tty`).
- Maintains continuous STDIN piping for interactive shell sessions (`--interactive`).
- Preserves full-screen interactive console properties.

## Usage Scenarios

### Basic Command Execution

The detailed sequence of parsing command line arguments and assembling the container's execution command is documented under [Argument Parsing Specification](./argument-parsing.md).

Representative execution commands:

```bash
# Given 'node' defined in .tools.yaml (e.g. image: node:20-alpine, entrypoint: ["node"]):
cderun node --version
# => Subcommand 'node' serves as the lookup key, resolving 'node:20-alpine' and entrypoint: ["node"].
#    The final command arguments forwarded to the container are '["--version"]', running 'node --version' inside.

# Specifying an explicit image and subcommand ('go' has no mapping in .tools.yaml):
# For tools where the desired command is not the image's default ENTRYPOINT, use the --entrypoint option.
cderun --image=golang:1.22 --entrypoint=go go --version
# => Subcommand 'go' is consumed as the lookup key (no profile match found). The image is 'golang:1.22',
#    and the ENTRYPOINT is overridden as 'go'.
#    The final container arguments are '["--version"]', running 'go --version' inside.
```

### Interactive Console Session

```bash
# Given 'bash' defined in .tools.yaml (e.g. image: node:20-alpine, entrypoint: ["bash"]):
cderun --tty --interactive bash
# => Subcommand 'bash' is the lookup key, resolving image 'node:20-alpine' and entrypoint 'bash'.
#    The final command argument list is empty, starting an interactive bash session in the container.

# Specifying a custom image and entrypoint on a tool with no profile mapping:
cderun --tty --interactive --image=alpine --entrypoint=sh sh
# => Subcommand 'sh' is consumed as the lookup key. The image is 'alpine' and ENTRYPOINT is overwritten as 'sh'.
#    An interactive 'sh' shell starts inside the container.
```

## Key Benefits

- **Zero Host Pollution**: Develop without installing multiple programming language runtimes or databases locally.
- **Environment Homogeneity**: Guarantee identical command behavior and versions across different developer environments and CI machines.
- **Workload Isolation**: Commands execute within sandbox containers, protecting the host system.
- **Transparent Execution**: Operates smoothly, mimicking the feel of locally installed commands.

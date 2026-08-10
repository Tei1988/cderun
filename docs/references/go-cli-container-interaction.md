# Developing Interactive CLI Tools inside Containers using Go: Architecture and Implementation Patterns In-Depth

In modern software development, container technologies represented by Docker and Podman have become an indispensable foundation. While these technologies have improved application portability, interacting with processes executed inside isolated environments requires a highly sophisticated interface design. To provide developers with a "transparent" experience, as if they were directly operating the shell inside the container from the host terminal, it is necessary to deeply understand and integrate the traditional pseudo-terminal (PTY) mechanisms of Unix-like operating systems and the concurrency model provided by the Go language.

This report comprehensively details the basic architecture, implementation details, signal propagation, window resize handling, and cross-platform considerations for building interactive CLI tools that communicate with processes inside containers using Go.

---

## Pseudo-Terminal (PTY) Fundamentals & Architecture

The core of implementing an interactive CLI lies in the concept of a Pseudo-Terminal (PTY). For normal program execution (e.g., running the `ls` command), it is sufficient to connect the standard input, standard output, and standard error streams using simple pipes. However, for applications like `vim`, `htop`, or interactive shells (`bash` or `zsh`) that control screen rendering or respond to specific key inputs (such as `Ctrl+C`), the execution environment must recognize that it is a "terminal (TTY)".

A PTY is a software-based device pair existing within the kernel, consisting of two endpoints: "Master" and "Slave". The Master side is held by the terminal emulator or the CLI tool itself, while the Slave side is connected to the child process, such as a shell. When transferring data between this pair, the kernel intercepts it via a processing layer called the "Line Discipline". This layer is responsible for processing input character display (echo back) and generating signals from specific control characters.

| Component | Role | Operational Overview |
| :---- | :---- | :---- |
| PTY Master | Endpoint controlled by the CLI tool | Writes user key inputs and reads output from the process. |
| PTY Slave | Endpoint recognized as a terminal by the child process | Assigned as standard I/O for the child process, which treats this endpoint as a real physical terminal. |
| Line Discipline | Data processing and conversion | Responsible for buffering input data, echo back, and signal generation via Ctrl+C, etc. |
| Kernel | Data mediator | Manages physical data transfer between master and slave, sending signals to the appropriate process groups. |

When executing a process inside a container, the CLI tool acts as a "relay", sending data read from the host-side PTY Master to the containerized process's standard input via the container engine's API, and conversely rendering the output from the containerized process onto the host-side terminal.

---

## I/O Relay Implementation in Go: os/exec and io.Copy

The standard way to launch an external process in Go is via the `os/exec` package. However, simply using `exec.Command` to start a process does not allocate a PTY to it. To achieve an interactive tool, a PTY must be explicitly allocated when starting the process. The most common library for this is `github.com/creack/pty`.

The `pty.Start(cmd)` function in `creack/pty` creates a new PTY Master/Slave pair, automatically assigns the Slave side to the standard input, standard output, and standard error of the `exec.Cmd`, and starts the process. It returns an `*os.File` representing the PTY Master, through which bidirectional communication can be conducted.

To connect this PTY Master with the host standard I/O (`os.Stdin`, `os.Stdout`), the powerful `io.Copy` abstraction is utilized. `io.Copy` continues copying data from a Reader to a Writer until EOF is reached. However, in interactive communication, "transferring user input" and "displaying process output" must be performed concurrently.

The general implementation pattern is as follows:

1. Start the process and PTY using `pty.Start`.
2. Spawn a goroutine to execute `io.Copy(ptyMaster, os.Stdin)` to forward keyboard inputs to the process.
3. In the main thread (or another goroutine), execute `io.Copy(os.Stdout, ptyMaster)` to render process output on the terminal.

Since `io.Copy` is a blocking operation, gracefully terminating these goroutines when the process exits is a critical challenge to prevent memory leaks and resource exhaustion.

---

## Terminal Control: Raw Mode and Standard I/O Management

By default, the terminal is in "Canonical Mode" (or "Cooked Mode"), where the operating system processes user inputs before passing them to the program. Specifically, input data is buffered until the user presses the "Enter" key, and corrections like backspaces are handled at the OS level. Additionally, "echoing" characters immediately on the screen is a feature of this mode.

When running an interactive shell or editor inside a container, this host-side processing interferes. For example, if the user presses `Ctrl+C` and the host OS intercepts it and terminates the CLI tool itself, the signal cannot be forwarded to the process inside the container. To solve this, the client terminal must be put into "Raw Mode".

In Raw Mode, all OS-level input processing and echoing are disabled. Every keypress is forwarded directly as a raw byte stream to the application (the Go CLI tool). This allows the application to detect arrow keys (special escape sequences) or control codes like `Ctrl+C` and relay them directly to the containerized process.

In Go, the best practice is to configure this using the `golang.org/x/term` package. The `term.MakeRaw` function modifies the terminal settings of the specified file descriptor (usually `os.Stdin`) and returns the original terminal state. Upon program exit, the terminal **must** be restored to its original mode using this saved state. Failure to restore the terminal will cause post-execution shells to display no characters or fail to handle newlines correctly, significantly degrading user experience.

| Flag (termios) | Effect (in Raw Mode) | Reason |
| :---- | :---- | :---- |
| ECHO | Disabled | Character echoing is handled by the containerized process; echoing on the host side would cause characters to be displayed twice. |
| ICANON | Disabled | Disables line buffering so that each character is sent to the process immediately upon entry. |
| ISIG | Disabled | Suppresses host-side signal generation from Ctrl+C or Ctrl+Z, allowing the application to receive them directly as data. |
| IEXTEN | Disabled | Disables extended input processing, treating all characters as pure input data. |

---

## Signal Handling: Ctrl+C, Ctrl+D, and Process Lifecycle

Signal handling in interactive applications is highly delicate. While Raw Mode suppresses automatic host-side signal generation, the application must detect user intent and forward it to the container.

### Distinguishing Ctrl+C (SIGINT) and Ctrl+D (EOF)

For most users, `Ctrl+C` means "abort the current operation", while `Ctrl+D` indicates "end of input (EOF)". In Raw Mode, when the user presses `Ctrl+C`, the application does not exit immediately; instead, it receives the raw byte `0x03` and writes it to the PTY Master. The line discipline on the PTY Slave interprets this byte and sends a `SIGINT` signal to the process running inside the container. This is how running commands can be aborted within the shell.

Conversely, `Ctrl+D` is not a signal, but represents the end of the input stream (EOF). In Go, when `0x04` is read from `os.Stdin` or `io.Copy` returns `io.EOF`, the application must close the input stream.

### Trapping and Forwarding SIGINT and SIGTERM

If the CLI tool itself receives a `SIGINT` or `SIGTERM` from an external source (e.g., when the tool runs in the background and is killed via the `kill` command), the tool is responsible for gracefully terminating the containerized process under its management rather than simply exiting.

In Go, these signals are monitored via a channel using `signal.Notify` from the `os/signal` package. The cleanup sequence upon receiving a signal is as follows:

1. Forward the signal to the process inside the container via the container engine (e.g., Docker API).
2. Wait for the process to exit within a specified grace period.
3. Restore the terminal state and exit the program.

The "PID 1 (initial process)" problem inside containers requires special attention. In a Docker container, the initial process runs as PID 1. According to Linux kernel behavior, PID 1 ignores signals unless explicit handlers are defined. To address this, users should enable the `docker run --init` flag or use `exec` in shell scripts to replace the PID 1 process.

---

## Window Resize Synchronization: SIGWINCH and PTY Adjustments

For terminal-based applications—especially those utilizing Terminal User Interfaces (TUI) like `vim` that occupy the entire screen—terminal row and column dimensions are critical. When a user resizes the terminal window, the host terminal driver sends a `SIGWINCH` (Signal Window Change) signal to the foreground process.

To propagate this change to the containerized process, the CLI tool must trap the `SIGWINCH` signal, fetch the new terminal size, and invoke an `ioctl` on the PTY Master to adjust its size.

### Implementation Steps

1. **Monitor Signals**: Set up a channel to receive `syscall.SIGWINCH` using `os/signal`.
2. **Fetch Current Size**: Invoke `TIOCGWINSZ` on the host terminal (such as `os.Stdout`) to get the current rows and columns.
3. **Apply to PTY**: Apply the retrieved dimensions to the PTY Master using functions like `pty.Setsize`.

```go
// Example of handling SIGWINCH
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGWINCH)
go func() {
    for range sigChan {
        // Fetch the host terminal size and propagate it to the PTY
        if err := pty.InheritSize(os.Stdin, ptyMaster); err != nil {
            log.Printf("failed to resize: %v", err)
        }
    }
}()
```

When this relay operates successfully, the kernel sends a `SIGWINCH` to the PTY Slave inside the container. The containerized process (e.g., `bash`) updates the `LINES` and `COLUMNS` environment variables or triggers screen re-rendering, preventing visual corruption.

---

## Cross-Platform Challenges: Unix PTY and Windows ConPTY

Supporting both Linux/macOS and Windows is a major challenge in CLI tool development. While Unix-like systems have established standard PTY mechanisms centered around `/dev/ptmx`, Windows lacked an equivalent mechanism for a long time.

### Windows ConPTY

Starting with Windows 10 Version 1809, Microsoft introduced the "Windows Pseudo Console (ConPTY)" API. While this enabled Unix-like PTY operations on Windows, its implementation model differs substantially. Instead of a simple master/slave pair, ConPTY establishes communication by passing two pipes ("input" and "output") created by the host application to the ConPTY instance.

| Feature | Unix-like (Linux/macOS) | Windows (ConPTY) |
| :---- | :---- | :---- |
| Device Model | `/dev/ptmx` device file | `CreatePseudoConsole` API call |
| Resizing | `ioctl(fd, TIOCSWINSZ, ...)` | `ResizePseudoConsole(handle, size)` |
| Signals | `SIGINT`, `SIGWINCH`, etc. | Console input events (or `win32-input-mode`) |
| Library Support | `creack/pty` is standard | Requires an abstraction layer like `aymanbagabas/go-pty` |

### Complexity of win32-input-mode

When running interactive applications in Windows Terminal, `win32-input-mode` is often enabled to achieve advanced control. In this mode, instead of simple ASCII characters, detailed escape sequences containing key down/up states and multiple modifier key states are transmitted. Building a Go CLI tool under this environment requires parsing these sequences into appropriate character data. Additionally, since `SIGWINCH` does not exist on Windows, window resizing must be detected via alternative methods (such as waiting for `WINDOW_BUFFER_SIZE_EVENT` in `ReadConsoleInput` or polling).

To simplify cross-platform support, it is wise to choose a library that abstracts operating system differences. `aymanbagabas/go-pty` supports both Unix PTY and Windows ConPTY, providing a unified interface that enhances code maintainability.

---

## Robust Cleanup and Resource Management: Preventing Goroutine Leaks

Goroutine leaks are one of the most common bugs in interactive CLI tools. They are particularly prone to occur when using `io.Copy` for I/O relaying.

### The io.Copy Blocking Problem

`io.Copy` blocks until the reading source returns EOF or a write error occurs on the destination. When the containerized process exits, the `io.Copy` handling the host standard input (`os.Stdin`) remains blocked on a `Read` operation until the user presses a key. This causes the goroutine to linger and consume resources indefinitely after the process has terminated.

### Non-blocking I/O and Context Mitigations

To prevent leaks and achieve robust termination, combine the following techniques:

1. **Force-Close on Process Exit**: Once `cmd.Wait()` detects process termination, immediately close the PTY Master file descriptor. This causes the `io.Copy` executing in other goroutines to detect a write error and exit.
2. **Context Cancellation**: Associate `context.Context` with long-running operations to ensure all related processes stop when the application terminates.
3. **Non-blocking Mode**: Use `syscall.SetNonblock` when necessary to configure timeouts on I/O operations, preventing infinite read blocks.

Additionally, during development, monitoring goroutine counts with `runtime.NumGoroutine()` or integrating tools like `uber-go/goleak` in the test suite helps identify leaks early.

### Exit Codes and Restoration

For scripting and automation, it is crucial that the CLI tool accurately propagates the containerized process's exit code to the host. In Go, the exit status is analyzed via the result of `cmd.Wait()`:

```go
err := cmd.Wait()
if err != nil {
    if exiterr, ok := err.(*exec.ExitError); ok {
        // Retrieve the exit status of the containerized process
        exitCode := exiterr.ExitCode()
        os.Exit(exitCode)
    }
}
```

Before calling `os.Exit`, the terminal must be restored to its normal state using `term.Restore`. The robust termination sequence should strictly follow:

1. Wait for process termination.
2. Restore terminal settings.
3. Release resources (close PTY, confirm goroutine termination).
4. Call `os.Exit` with the retrieved exit code.

---

## Practical Examples and Library Utilization

Leveraging high-quality existing libraries is highly effective to avoid reinventing the wheel.

### 1. aymanbagabas/go-pty

This library is a powerful tool to hide differences between Unix PTY and Windows ConPTY. Its Windows support is exceptionally robust, automatically managing ConPTY attributes that are difficult to configure via standard `os/exec` alone. It is the premier choice for cross-platform interactive CLI tools.

### 2. golang.org/x/term

An extension of the standard library, providing functions for entering raw mode (`MakeRaw`) and retrieving terminal sizes (`GetSize`). It is ideal when you want to minimize external dependencies and utilize OS-native features reliably.

### 3. Docker Go SDK (moby/moby)

When communicating directly with the container engine, developers can use the SDK directly instead of executing the external `docker exec` binary. Using `ContainerExecCreate` and `ContainerExecAttach` from the SDK establishes a direct stream (`net.Conn`) to the process inside the container. Feeding this stream into the PTY handling logic enables highly integrated tool design.

| Requirement | Recommended Approach | Reason |
| :---- | :---- | :---- |
| Minimal Dependencies | `os/exec` + `creack/pty` | Extremely lightweight and stable under Linux/Unix. |
| Full Windows Support | `aymanbagabas/go-pty` | Wraps the complex Windows ConPTY API into an easy-to-use, `exec.Cmd`-like Go interface. |
| Advanced TUI | `charmbracelet/bubbletea` | Ideal for combining PTY handling with sophisticated terminal rendering frameworks. |
| Direct Engine Integration | Docker SDK | Eliminates external binary dependencies and facilitates remote container operations. |

---

## Conclusion: Towards Robust Interactive Tools

Building an interactive CLI tool that communicates with containerized processes in Go goes beyond simple programming; it is a system engineering challenge that bridges low-level OS features with container technologies. If any of the PTY architecture, Raw Mode control, signal propagation, or window resize synchronization is missing, the user experience will be compromised.

Particularly in modern environments, cross-platform compatibility and rigorous resource management (preventing goroutine leaks) are key differentiators. Understanding the blocking behavior behind `io.Copy` and controlling it in alignment with the process lifecycle is the key to creating professional CLI tools.

By adhering to the patterns and best practices detailed in this report, you can provide users with a seamless and powerful interactive experience that feels as if container boundaries do not exist, while maintaining the isolation benefits of Docker and Podman environments.

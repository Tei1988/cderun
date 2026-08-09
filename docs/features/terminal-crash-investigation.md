# Technical Investigation Report: Terminal Crash via TTY Execution (T01)

## Overview

This document presents the technical investigation into the terminal crash issue observed when executing interactive CLI tools (such as `kiro-cli`) via `cderun` under macOS Terminal (Terminal.app). When the terminal cursor reaches the rightmost edge, the host terminal emulator crashes or terminates abruptly.

---

## Technical Analysis

The investigation identified two main intersecting areas of terminal handling in `cderun` that contribute to this phenomenon:

### 1. Terminal Raw Mode and Stream Architecture

- **Raw Mode Initialization:** When `cc.TTY` is requested, `cderun` invokes `setupTerminal()`, putting the host's standard input file descriptor (`stdinFd`, typically `0`) into raw mode via `term.MakeRaw(stdinFd)`.
- **PTY Stream Multiplexing:** Inside the container runtimes (Docker, containerd), a virtual pseudo-terminal (PTY) is allocated. Output from the container is streamed verbatim to the host terminal's standard output via `io.Copy`. The host terminal is responsible for interpreting raw ANSI/VT100 escape sequences.

### 2. Asynchronous Window Resize Tracking (`SIGWINCH`)

`cderun` runs a dedicated background goroutine to handle `SIGWINCH` resize events via `startResizeHandler()`:

```go
handleResize := func() {
  w, h, err := o.termGetSize(fd)
  if err == nil && h >= 0 && w >= 0 {
    _ = rt.ResizeContainerTTY(ctx, containerID, uint(h), uint(w))
  }
}
```

This updates the virtual PTY dimensions inside the container runtime to match the host terminal dimensions.

---

## Suspected Root Cause Analysis (Hypothesized Mechanisms)

While a definitive confirmation requires a full crash log or thread backtrace from macOS Terminal.app, the abrupt termination of the terminal emulator is strongly suspected/hypothesized to be caused by a geometry and stream synchronization mismatch when cursor wrapping occurs under raw mode, specifically exacerbated by the following mechanisms:

### 1. Hypothesized DSR/Wrap Race and Layout-Thread Mismatch

- **Query-Resize Mismatch:** Fully-interactive CLI editors (such as `kiro-cli` or editors using Rust's `crossterm` / `termion` crates) periodically query the terminal state using the Device Status Report sequence (`\x1b[6n`) to obtain precise cursor coordinates.
- **Race Condition:** If there is a transient mismatch where the container PTY is slightly larger/smaller than the physical host window, or if a resize event is in-flight, the editor may calculate rendering offsets based on stale geometry.
- **Suspected Terminal Crash Trigger:** It is hypothesized that if a wrap sequence or rendering command is sent to macOS Terminal.app when the cursor is precisely at the right edge boundary (the auto-wrap DECAWM boundary) under raw mode with misaligned row/column settings, the terminal's internal layout thread may encounter a rendering buffer boundary mismatch, leading to an uncaught `SIGSEGV` or exception in Terminal.app itself.

### 2. Confirmed Zero-Geometry Loop and Suspected Flood Trigger

- **Confirmed cderun Resize Behavior:** During window layout or minimization transitions, `termGetSize` can temporarily return `0` for height or width. Currently, `uint(h)` and `uint(w)` are passed directly to the PTY without non-zero validation.
- **PTY Loop Behavior:** When the PTY column width is set to `0`, containerized applications are confirmed to enter division-by-zero loops or start a draw loop where they flood the standard output stream with invalid positioning sequences like `\x1b[y;0H`.
- **Suspected Crash Trigger:** The receipt of high-frequency invalid drawing operations or corrupt sequences at the wrap boundaries is suspected to exceed the boundary tolerance of Terminal.app, triggering the app's termination.

---

## Reproduction Steps

1. **Environment:** macOS with Terminal.app.

2. **Execution:** Run cderun with TTY/Interactive enabled to start an interactive shell:

   ```bash
   cderun --tty --interactive --image=rust sh
   ```

3. **Trigger:** Inside the container, run a terminal editor like `kiro-cli` or execute a command that draws characters rapidly up to the exact column width of the window.

4. **Action:** Resize the window quickly or cause the cursor to reach the rightmost column edge.

5. **Result:** macOS Terminal.app terminates immediately.

---

## Recommended Mitigations and Solutions

To harden the runner and protect the host terminal from crashing, we propose the following changes:

### 1. Minimum Geometry Validation (Skip Invalid Geometry)

Ensure that PTY size updates never pass `0` or negative values to the runtime by validating the dimensions and skipping invalid sizes:

```go
handleResize := func() {
  w, h, err := o.termGetSize(fd)
  if err == nil && h > 0 && w > 0 { // Skip 0 or negative coordinates
    _ = rt.ResizeContainerTTY(ctx, containerID, uint(h), uint(w))
  }
}
```

### 2. Debouncing / Coalescing SIGWINCH Events

Introduce a small debounce delay (e.g., 50ms) to the `SIGWINCH` resize loop. This prevents sending multiple intermediate PTY resize requests to the container runtime during drag-resizing, ensuring smooth drawing and synchronized geometry updates.

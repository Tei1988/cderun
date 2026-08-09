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

## Root Cause Analysis

The abrupt termination/crash of macOS Terminal.app is caused by a race condition and geometry mismatch when cursor wrapping occurs under raw mode, exacerbated by boundary handling:

### 1. DSR Cursor Position Query (Device Status Report) and Wrap Out-of-Bounds

- Fully-interactive CLI editors (such as `kiro-cli` or editors using Rust's `crossterm` / `termion` crates) periodically query the terminal state using the Device Status Report sequence (`\x1b[6n`) to get precise cursor coordinates.
- If there is a transient mismatch where the container PTY is slightly larger/smaller than the physical host window, or if a resize event is in-flight, the editor can calculate coordinates based on stale geometry.
- If a wrap sequence or rendering command is sent to macOS Terminal.app when the cursor is precisely at the right edge boundary (the auto-wrap DECAWM boundary) with misaligned row/column settings, the terminal's internal layout thread encounters a rendering buffer boundary mismatch, leading to an uncaught crash (SIGSEGV) in Terminal.app itself.

### 2. Zero-Geometry Division-by-Zero and Loop Flood

- When window size is extremely small, or during layout/minimization transitions, `termGetSize` can temporarily return `0` for height or width.
- `uint(h)` and `uint(w)` are passed to the PTY. If the column width is set to `0`, containerized applications enter division-by-zero panics or start an infinite drawing loop where they flood the standard stream with invalid positioning sequences like `\x1b[y;0H`.
- Receipt of high-frequency corrupt sequences or invalid coordinate operations at the right edge wraps the rendering boundary in Terminal.app and triggers the emulator crash.

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

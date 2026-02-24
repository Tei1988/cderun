# Standard Input Synchronization

This document explains the synchronization mechanism for standard input (STDIN) in `cderun` and why it is necessary for reliable piped execution.

## The Problem: Race Conditions in Piped Execution

When running a command like `echo "test" | cderun ... cat`, there is a race condition between the following events:

1. **STDIN Attachment**: `cderun` calls Docker's `AttachContainer` API to connect the host's STDIN to the container.
2. **Container Start**: `cderun` calls Docker's `StartContainer` API to actually begin execution of the command (e.g., `cat`).
3. **STDIN Consumption**: The command inside the container (e.g., `cat`) starts reading from its STDIN.

If the host's STDIN is a pipe (like from `echo`), the data is available immediately. If `cderun` starts copying this data to the container's input stream *before* the container has actually started, some Docker versions or configurations might drop the data or fail to deliver it correctly to the process when it eventually starts.

Furthermore, if the data is small (like "test\n"), `cderun` might finish copying all data and call `CloseWrite()` on the connection before the containerized process has even had a chance to open its own STDIN. This often results in the process seeing an immediate EOF and exiting (or in some cases, never seeing the data at all and hanging).

## The Solution: Synchronized STDIN

To ensure reliable piped input, `cderun` implements a synchronization mechanism using a `syncReader`.

### 1. Delayed STDIN Reading

`cderun` wraps the host's STDIN in a `syncReader` before passing it to the runtime's `AttachContainer` method. This reader blocks any `Read` calls until it receives a signal.

### 2. Signal on Container Start

The signal to unblock the `syncReader` is sent only *after* the `StartContainer` API call has successfully returned. This guarantees that the container is officially "running" before any data from the host's STDIN is pushed into the container's input stream.

```go
// internal/command/root.go

type syncReader struct {
	inner io.Reader
	ready <-chan struct{}
	ctx   context.Context
}

func (s *syncReader) Read(p []byte) (n int, err error) {
	select {
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	case <-s.ready:
		return s.inner.Read(p)
	}
}
```

### 3. Disabling Logs in Attach

In `internal/runtime/docker.go`, the `AttachContainer` call now sets `Logs: false`.

Since `cderun` always attaches to the container *before* starting it, there are no existing logs to fetch. Setting `Logs: true` can sometimes cause the Docker daemon to send an initial (empty) log stream and then close or misbehave if the container hasn't started yet, especially under heavy load or specific Docker versions. Disabling it ensures a cleaner stream connection dedicated to real-time IO.

## Benefits

- **Reliability**: Piped input works consistently even for very fast-executing commands or small data sets.
- **No Data Loss**: Data is only sent when the container is ready to receive it.
- **Correct EOF Handling**: The EOF from the host's STDIN is delivered to the containerized process at the correct time.

## Verification

This behavior is verified by unit tests in `internal/command/stdin_test.go` which simulate delayed container startup with immediate STDIN availability.

## Automatic Termination for Docker 29.1.5 Compatibility

In some Docker versions (notably 29.1.5), a hang can occur during piped execution where the containerized process has consumed all input and produced all output, but the container remains in a `Running` state and the `WaitContainer` API call does not return.

To address this, `cderun` implements an automatic termination logic for non-TTY executions:

1. **Concurrent Waiting**: `cderun` waits for both container exit (`WaitContainer`) and IO completion (`AttachContainer`) concurrently.
2. **IO Completion Detection**: When the output stream from the container is closed (EOF reached), `AttachContainer` returns.
3. **Hang Timeout**: If IO is complete but the container does not exit within a graceful timeout (**2 seconds** for TTY, **100ms** for pipes/files), `cderun` assumes a hang has occurred and sends a `SIGKILL` to the container.
4. **Preserving Interactivity**: This automatic termination is **disabled** when TTY is requested (`--tty` or `-t`), ensuring that interactive shells or long-running UI applications are not prematurely killed.

This ensures that piped commands like `echo "data" | cderun cat` always exit promptly after their work is done.

## Docker 29.1.5 互換性のための自動終了処理

Docker 29.1.5 などの一部のバージョンでは、パイプ実行時にコンテナ内のプロセスがすべての入力を消費し、出力を完了したにもかかわらず、コンテナが `Running` 状態のままとなり `WaitContainer` API が戻ってこない（ハングする）事象が確認されています。

この問題に対処するため、`cderun` は非 TTY 実行時に以下の自動終了ロジックを実装しています。

1. **並行待機**: コンテナの終了（`WaitContainer`）と IO の完了（`AttachContainer`）を並行して待機します。
2. **IO 完了の検知**: コンテナからの出力ストリームが閉じられる（EOF 到着）と、`AttachContainer` が終了します。
3. **ハングタイムアウト**: IO が完了したにもかかわらず、入力種別に応じた猶予期間内（TTYでは**2秒**、パイプ/ファイルでは**100ms**）にコンテナが終了しない場合、`cderun` はハングが発生したとみなし、コンテナに `SIGKILL` を送信します。
4. **インタラクティブ性の維持**: この自動終了ロジックは、TTY が要求されている場合（`--tty` または `-t`）は**無効**になります。これにより、インタラクティブなシェルや長時間実行される UI アプリケーションが誤って終了されるのを防ぎます。

これにより、`echo "data" | cderun cat` のようなパイプ実行時に、処理完了後すぐに CLI が終了することが保証されます。

## Dynamic Hang Timeout based on STDIN Type

`cderun` uses a dynamic timeout mechanism to handle the Docker hang issue (where a container doesn't exit after all IO is done) while maintaining responsiveness and interactivity.

### TTY vs Pipe/File Detection

Note: In this document, "TTY" is used in two different contexts:

1. **The `--tty` or `-t` flag**: Controls whether a pseudo-TTY is allocated for the container. When this flag is set, automatic termination (SIGKILL) is **disabled** to preserve interactivity.
2. **Host STDIN status**: Whether the host's standard input is a terminal. This determines the **Hang Timeout** duration (2s for TTY, 100ms for pipes/files).

`cderun` detects whether the host's STDIN is a terminal (TTY) or a pipe/file.

- **Pipe or File Redirect**: If the input is piped (e.g., `echo "data" | cderun ...`) or redirected from a file, `cderun` assumes a batch/pipeline mode.
- **TTY (Terminal)**: If the input is a terminal, `cderun` assumes an interactive session.

### Behavior Differences

After the IO operations are completed (both STDIN copy and output stream reach EOF), `cderun` waits for the container to exit naturally.

1. **In Pipeline Mode (Pipe/File)**:
   - Responsibility is key: The user expects the command to finish immediately after the data has been processed.
   - **Hang Timeout**: Reduced to **100ms**.
   - If the container doesn't exit within 100ms after IO completion, `cderun` sends `SIGKILL` to ensure the pipeline continues promptly.

2. **In Interactive Mode (TTY)**:
   - Context is key: The user is interacting with the process. Even if output stops, the user might still be typing or the process might be doing some final cleanup.
   - **Hang Timeout**: Maintained at **2 seconds**.
   - This provides a graceful period for the container to exit naturally before forcing termination.

3. **Non-Interactive Flag (`-i` not set)**:
   - If `--interactive=false` is explicitly set (or default), `cderun` also uses the short **100ms** timeout, as it won't be reading any user input anyway.

This optimization ensures that commands like `echo "test" | cderun cat` exit instantly while `cderun -it alpine sh` remains robust and user-friendly.

## STDIN タイプに基づく動的なハングタイムアウト

`cderun` は、Docker のハング問題（全ての IO が完了した後にコンテナが終了しない問題）に対処しつつ、応答性とインタラクティブ性を維持するために、動的なタイムアウトメカニズムを使用しています。

### TTY とパイプ/ファイルの判定

補足: このドキュメントでは「TTY」という言葉を2つの異なる文脈で使用しています：

1. **`--tty` または `-t` フラグ**: コンテナに疑似ターミナルを割り当てるかどうかを制御します。このフラグが設定されている場合、対話性を優先するため自動終了ロジック（SIGKILL）は**無効**になります。
2. **ホスト標準入力の状態**: ホストの標準入力が端末かどうかを指します。これは**ハングタイムアウト**の長さを決定します（TTYなら2秒、パイプ/ファイルなら100ms）。

`cderun` は、ホストの標準入力（STDIN）が端末（TTY）であるか、パイプ/ファイルであるかを判定します。

- **パイプまたはファイルリダイレクト**: 入力がパイプ（例: `echo "data" | cderun ...`）またはファイルからリダイレクトされている場合、`cderun` はバッチ/パイプラインモードであるとみなします。
- **TTY (端末)**: 入力が端末である場合、`cderun` は対話型セッションであるとみなします。

### 挙動の違い

IO 操作が完了した後（STDIN のコピーと出力ストリームの両方が EOF に達した後）、`cderun` はコンテナが自然に終了するのを待ちます。

1. **パイプラインモードの場合 (パイプ/ファイル)**:
   - 応答性が重要です。ユーザーはデータ処理が完了した後、コマンドがすぐに終了することを期待します。
   - **ハングタイムアウト**: **100ms** に短縮されます。
   - IO 完了後 100ms 以内にコンテナが終了しない場合、`cderun` は `SIGKILL` を送信し、パイプラインが速やかに継続されるようにします。

2. **対話型モードの場合 (TTY)**:
   - コンテキストが重要です。ユーザーはプロセスと対話しています。出力が止まっても、ユーザーがまだ入力中であったり、プロセスが最終的なクリーンアップを行っていたりする可能性があります。
   - **ハングタイムアウト**: **2秒** を維持します。
   - これにより、強制終了する前にコンテナが自然に終了するための猶予期間が提供されます。

3. **非対話型フラグ (`-i` が設定されていない場合)**:
   - `--interactive=false` が明示的に設定されている（またはデフォルトの）場合も、ユーザー入力を読み取らないため、短い **100ms** のタイムアウトを使用します。

この最適化により、`echo "test" | cderun cat` のようなコマンドは瞬時に終了し、一方 `cderun -it alpine sh` は堅牢でユーザーフレンドリーな動作を維持します。

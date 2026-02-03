# インタラクティブ・ターミナル (Completed)

この機能は、`cderun` における堅牢なインタラクティブ・ターミナル・サポートを実装し、コンテナ内でインタラクティブ・シェルや TUI アプリケーションを実行する際に、シームレスで「透過的な」体験を保証します。

実装は、[docs/references/go-cli-container-interaction.md](../references/go-cli-container-interaction.md) で説明されている技術的パターンに基づいています。

## 機能概要

### 1. ターミナル Raw モード (Completed)
TTY が有効な場合（`--tty`）、ホストのターミナルは `golang.org/x/term` を使用して「Raw モード」に設定されます。これにより、ローカルのエコーや行バッファリングが無効になり、すべてのキーストローク（コントロールキャラクターを含む）がコンテナ化されたプロセスに直接送信されます。ターミナルの状態は終了時に自動的に復元されます。

### 2. シグナルの処理と転送 (Completed)
`cderun` はホストで受信したライフサイクルシグナル（`SIGINT`, `SIGTERM`）を捕捉し、コンテナランタイム API 経由でコンテナ化されたプロセスに転送します。これにより、`Ctrl+C` を押したり `cderun` に終了シグナルを送ったりした際に、コンテナ内のプロセスが正しくクリーンアップされることが保証されます。また、停止しないコンテナのための二段階終了もサポートしています。

### 3. ウィンドウリサイズの同期 (SIGWINCH) (Completed)
`cderun` はホストターミナルのウィンドウリサイズイベント（`SIGWINCH`）を監視します。ターミナルがリサイズされると、新しいディメンション（行と列）がコンテナの TTY と動的に同期され、`vim` や `htop` のような TUI アプリケーションでの表示崩れを防ぎます。

### 4. 堅牢な I/O 管理とクリーンアップ (Completed)
ゴルーチンリークを防ぐために I/O ストリームが管理されています。コンテナが終了すると接続は適切に閉じられ、すべてのバックグラウンドリレーゴルーチンが正しく終了することを保証します。

### 5. Windows ConPTY サポート (将来)
Windows ホストで一貫したインタラクティブ体験を提供するために、将来のフェーズで Windows Pseudo Console (ConPTY) のサポートが計画されています。

## 実装の詳細
- **Raw モード**: `term.MakeRaw(int(os.Stdin.Fd()))`
- **シグナル転送**: `os/signal` および `Runtime.SignalContainer`
- **リサイズ**: `SIGWINCH`（プラットフォーム固有のファイル `signals_unix.go` および `signals_windows.go` で処理）および `Runtime.ResizeContainerTTY`

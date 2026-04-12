# 標準入力の同期 (Standard Input Synchronization)

このドキュメントでは、`cderun` における標準入力（STDIN）の同期メカニズムと、なぜそれが信頼性の高いパイプ実行に必要なのかを説明します。

## 問題点: パイプ実行におけるレースコンディション

`echo "test" | cderun ... cat` のようなコマンドを実行すると、以下のイベント間でレースコンディション（競合状態）が発生します。

1. **STDINのアタッチ**: `cderun` が Docker の `AttachContainer` API を呼び出し、ホストの STDIN をコンテナに接続する。
2. **コンテナの開始**: `cderun` が Docker の `StartContainer` API を呼び出し、実際にコマンド（例: `cat`）の実行を開始する。
3. **STDINの消費**: コンテナ内のコマンド（例: `cat`）が自身の STDIN からの読み取りを開始する。

ホストの STDIN がパイプ（`echo` などから）である場合、データは即座に利用可能です。`cderun` がコンテナの開始**前**にこのデータをコンテナの入力ストリームにコピーし始めると、一部の Docker バージョンや構成では、データが欠落したり、プロセスが開始されたときに正しく配信されなかったりすることがあります。

さらに、データが小さい場合（例: "test\n"）、コンテナ化されたプロセスが自身の STDIN を開く機会を得る前に、`cderun` がすべてのデータのコピーを完了し、接続に対して `CloseWrite()` を呼び出してしまう可能性があります。これにより、プロセスが即座に EOF を検知して終了してしまったり（あるいは、データが全く見えずハングしたり）することがよくあります。

## 解決策: 同期された STDIN

信頼性の高いパイプ入力を保証するために、`cderun` は `syncReader` を使用した同期メカニズムを実装しています。

### 1. 遅延 STDIN 読み取り

`cderun` は、ホストの STDIN を `syncReader` でラップしてから、ランタイムの `AttachContainer` メソッドに渡します。このリーダーは、信号を受け取るまで `Read` 呼び出しをブロックします。

### 2. コンテナ開始時の信号

`syncReader` のブロックを解除する信号は、`StartContainer` API 呼び出しが正常に返された**後**にのみ送信されます。これにより、ホストの STDIN からのデータがコンテナの入力ストリームに送られる前に、コンテナが公式に「実行中（running）」であることが保証されます。

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

### 3. StdinOnce による確実な EOF 伝達

`internal/runtime/docker.go` において、`Interactive` モードが有効な場合に `StdinOnce: true` を設定しています。

Docker のデフォルト挙動（`StdinOnce: false`）では、クライアントが標準入力を閉じても、Docker デーモン側でコンテナの入力ストリームを維持し続けることがあります。特に非 TTY かつパイプ入力の場合、これによりコンテナ内のプロセス（`cat` など）が EOF を検知できず、終了しない原因となります。

`StdinOnce: true` を設定することで、`cderun` が標準入力のコピーを完了して接続を閉じた際に、Docker が確実にコンテナへ EOF を伝え、プロセスが正常に終了（Exit 0）することを保証します。

### 4. CloseWrite 前の猶予期間 (Docker)

Docker ランタイム実装 (`internal/runtime/docker.go`) では、STDIN の全データコピーが完了した後、`CloseWrite` を呼び出す前に **1秒間** の猶予期間 (`attachCloseWriteGrace`) を設けています。これにより、非常に小さなデータがコンテナ内のプロセスに到達する前に接続が切断されるのを防ぎ、Docker デーモンが確実にデータを処理する時間を確保します。

### 5. Attach 時のログ取得の無効化

`internal/runtime/docker.go` において、`AttachContainer` 呼び出し時に `Logs: false` を設定しています。

`cderun` は常にコンテナを開始する前にアタッチするため、取得すべき既存のログは存在しません。`Logs: true` を設定すると、特に高負荷時や特定の Docker バージョンにおいて、コンテナがまだ開始されていない場合に Docker デーモンが初期（空の）ログストリームを送信し、その後接続を閉じたり誤動作したりすることがあります。これを無効にすることで、リアルタイム IO に特化したクリーンなストリーム接続が確保されます。

## メリット

- **信頼性**: 非常に高速に実行されるコマンドや小さなデータセットに対しても、パイプ入力が一貫して動作します。
- **データ欠損なし**: コンテナがデータを受け取る準備ができてからデータが送信されます。
- **正確な EOF 処理**: ホストの STDIN からの EOF が、適切なタイミングでコンテナ化されたプロセスに配信されます。

## 検証

この挙動は `internal/command/stdin_test.go` のユニットテストによって検証されています。このテストでは、コンテナの起動を遅らせ、即座に利用可能な STDIN をシミュレートしています。

## タイムアウトによる自動終了処理

`cderun` は、特定の環境下で発生しうるコンテナのハングに対処するため、非 TTY または非インタラクティブ実行時に自動終了（ハングタイムアウト）ロジックを実装しています。

詳細については、[ハングタイムアウト](./hang-timeout.md) を参照してください。

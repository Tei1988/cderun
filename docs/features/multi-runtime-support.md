# 機能仕様：マルチランタイムサポート (完了)

## 概要

Docker以外のコンテナランタイム（Podman等）をサポートする。
共通の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

## サポートされるランタイム

### 優先度1: Docker (完了)

- デフォルトのランタイム
- 最も広く使われている
- Docker Engine APIを使用

### 優先度2: Podman (完了)

- Dockerのドロップイン代替
- rootlessコンテナのサポート
- Podman APIを使用（Docker互換）

### サポートされているランタイム

- **containerd (Experimental)**:
  ネイティブの containerd API (gRPC) を直接利用したサポートを開発中です。
  - **目的**: Docker/Podman デーモンを経由しない、より軽量なコンテナ実行の実現。
  - **現状**: 基本的な実行機能は実装されていますが、以下の制限事項があります。
    - **ネットワーク**: `host` ネットワークのみをサポートしています（デフォルトの `bridge` ネットワークは未サポート）。
    - **ポート公開**: ポートマッピング（`--publish`, `-p`, `--publish-all`, `-P`）およびポート公開（`--expose`）は未サポートです。
    - **DNS/ホスト**: カスタムDNSサーバの設定（`--dns`）およびホストマッピングの追加（`--add-host`）は未サポートです。
    - **マウント**: `volume` タイプ（名前付きボリューム等）のマウントは未サポートです（`bind` または `tmpfs` を使用してください）。
    - **ケーパビリティ**: `--cap-add`, `--cap-drop` による Linux ケーパビリティの制御はサポートされています。containerd アダプターは、Docker互換のケーパビリティ名（例: `SYS_ADMIN`）を OCI-spec-compliant な名称（例: `CAP_SYS_ADMIN`）へと自動的に正規化します。
    - **ENTRYPOINTの継承**: イメージに `ENTRYPOINT` が定義されている場合、コマンド指定時にもデフォルトの `ENTRYPOINT` が自動的に前置して適用されるようになり、Docker ランタイムと同様の挙動が保証されています。
    - **プラットフォーム制限**: **Linux専用**です（`//go:build linux` ビルドタグ付き）。macOSやWindowsでは、DockerやPodmanのように仮想マシン内で動作するエンジンを経由せずに直接ローカルの containerd に接続することはできません。
- **nerdctl (Backlog)**:
  containerd の CLI である `nerdctl` をラップして実行する方式の検討。

## アーキテクチャ

### 抽象化レイヤー

cderun独自の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

```text
cderun ContainerRuntimeインターフェース
        │
        ├── DockerRuntime → Docker Engine API (HTTP over Unix socket)
        ├── PodmanRuntime → Podman API (HTTP over Unix socket)
        └── ContainerdRuntime → containerd API (gRPC)
```

### 共通インターフェースの役割

`ContainerRuntime` インターフェースは、以下の主要な責務を持つ：

- **ライフサイクル管理**: コンテナの作成、起動、終了待機、削除。
- **IO接続**: コンテナの標準入出力へのアタッチ（TTYサポート含む）。
- **メタデータ提供**: ランタイム名の識別。
- **操作**: コンテナへのシグナル送信、TTYリサイズ。

## ランタイムの選択

**現状 (Phase 4):**
Docker と Podman をフルサポートしています。Podman は Docker 互換の API を介してサポートされており、ランタイムとソケットの選択は、設定ファイル、環境変数、またはコマンドライン引数によって明示的に指定可能です。

### 解決ロジック (完了)

1. **設定ファイル**: `.cderun.yaml` の `runtime` フィールド。
2. **環境変数**: `CDERUN_RUNTIME`, `CDERUN_SOCKET_PATH` 等。
3. **コマンドライン引数**: `--runtime`, `--socket-path` および P1 内部オーバーライド。

### 自動検出ロジック (完了)

ソケットの存在確認によるランタイムの自動選択機能。

1. `--runtime` または `CDERUN_RUNTIME` が指定されている場合はそれを使用。
2. 指定がない場合、以下のデフォルトパスを順に確認し、最初に見つかったものを使用。

  - `/var/run/docker.sock` (Runtime: `docker`)
  - `/run/containerd/containerd.sock` (Runtime: `containerd`)
  - `/run/podman/podman.sock` (Runtime: `podman`)

3. いずれも見つからない場合は `docker` をデフォルトとし、`/var/run/docker.sock` を使用（実行時にエラーとなる可能性がある）。

#### パフォーマンスとキャッシュ設計 (Socket Detection Cache)

`cderun` は、設定解決における不要なディスク I/O や `Stat` システムコールを削減するため、ソケット自動検出結果に対してインテリジェントなキャッシュ機構を備えています。

- **実ファイルシステムのキャッシュ化**: 実行ホストの実際のファイルシステム（`RealFileSystem`）を使用している場合、初回に成功したソケット自動検出結果は、グローバルな書き込み/読み込みロック（`sync.RWMutex`）によって保護されたプロセス生存期間キャッシュに保存されます。以降の構成解決（Resolution）処理では、このキャッシュされた結果を直接返すことで、システムコールを完全にバイパスします。
  - **制限事項と動作**: 初回検出に成功した後は、プロセスの生存期間中にソケットの存在確認や再検証は行われません。そのため、自動検出後にソケットファイルが消失したり、有効なコンテナランタイムが切り替わったりしても古い検出結果（キャッシュ）が使用され続けます。キャッシュを無効化・更新して最新のソケットを検出させるためには、プロセス自体を再起動するか、あるいはコマンドライン引数（`--runtime` / `--socket-path`）、環境変数（`CDERUN_RUNTIME` / `CDERUN_SOCKET_PATH`）、もしくは設定ファイル（`runtime` / `socketPath`）で明示的な上書き指定（overrides）を行う必要があります。これら明示的な上書き指定がある場合は、自動検出キャッシュを完全にバイパスして指定されたパスやランタイムが使用されます。
- **動的再検出（フォールバック）**: 自動検出時に有効なソケットパスが1つも見つからなかった場合は、キャッシュに登録されません。これは、実行中にバックグラウンドでコンテナデーモンが起動された場合に備え、次回の解決処理時にも再度検出プローブを実行できるようにするためです。
- **テストの隔離と安全設計**: モックやインメモリファイルシステム（`MockFileSystem` など）を用いたテスト実行時においては、テスト間の状態漏洩や競合を防ぐため、キャッシュへの書き込み・読み込みは意図的にスキップされ、常に動的な検出プローブが実行されます。

### 明示的な指定 (完了)

#### 設定ファイル (`.cderun.yaml`)

```yaml
runtime: podman
```

#### 環境変数

```bash
export CDERUN_RUNTIME=podman
export CDERUN_SOCKET_PATH=/run/podman/podman.sock
cderun node app.js
```

#### コマンドライン

```bash
cderun --runtime podman node app.js
```

## ランタイム固有の実装ポイント

- **Docker**: `github.com/docker/docker/client` を使用し、Unixソケット経由で接続。APIバージョンの自動ネゴシエーションを有効化。
- **Podman**: Docker 互換の API を使用。 Docker クライアントライブラリを共通の基盤として利用し、Podman の Unix ソケット経由で接続。
- **イメージプルのリトライ**: ネットワークの不安定さやレート制限（`toomanyrequests` 等）に対応するため、指数バックオフを伴うリトライロジック（最大3回）を実装。

## ランタイム情報の表示 (完了)

### 現在のランタイム確認

`--diagnosis` フラグを使用することで、現在のランタイム設定や診断情報を表示できます。詳細は[診断モード](./diagnosis-mode.md)を参照してください。

```bash
cderun --diagnosis
```

`handleDiagnosis` が報告する診断情報（`--diagnosis` の出力結果）には、以下の項目のみが含まれます。

- **ランタイム名**: 現在のアクティブなコンテナランタイム名（`docker` / `podman` / `containerd`）。
- **ソケットパスとそのアクセス可能性**: 解決されたソケットパスと、そのソケットがホスト上に存在してアクセス可能かどうか。これはデーモンの直接的な接続/疎通確認ではなく、ホスト側ファイルシステムの検査（`fs.Stat`）によって検証されます。
- **設定ファイルのパス**: 解決された `.cderun.yaml` （グローバル設定）および `.tools.yaml` （ツールマッピング設定）ファイルの絶対パス。
- **利用可能なツール**: `.tools.yaml` 内に定義されている有効なツール名の一覧。

診断情報の出力フォーマットは `--diagnosis-format` で指定可能です（`yaml`, `json`, `simple`）。

```bash
cderun --diagnosis --diagnosis-format simple
```

## 拡張性

### 新しいランタイムの追加手順

1. `ContainerRuntime` インターフェースを実装する新しい構造体を作成。
2. `internal/command/root.go` の `runtimeFactory` に新しいランタイムを登録。
3. 設定ファイルや環境変数で新しいランタイムを選択可能にする。

## 依存ライブラリ

### Docker

```go
import (
    "github.com/docker/docker/client"
)
```

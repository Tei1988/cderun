# Feature: Direct Container Execution (Completed)

## 概要

各ランタイムの独自APIを介して直接コンテナを実行する。
コマンド生成は行わず、中間表現（IR）を各ランタイムのAPIコールに変換する。

## アーキテクチャ

```
cderunフラグ → 中間表現（IR） → ランタイムAPIコール → コンテナ実行
                    ↓
               ContainerConfig
                    ↓
          runtime.CreateContainer()
          runtime.StartContainer()
          runtime.AttachContainer()
          ...
```

### 実装ステータス (CRIインターフェース)

| メソッド | Docker (moby) | Podman (bindings) |
| :--- | :---: | :---: |
| `CreateContainer` | implemented | stub / planned |
| `StartContainer` | implemented | stub / planned |
| `WaitContainer` | implemented | stub / planned |
| `RemoveContainer` | implemented | stub / planned |
| `AttachContainer` | implemented | stub / planned |
| `SignalContainer` | implemented | stub / planned |
| `ResizeContainerTTY` | implemented | stub / planned |

**メリット:**
- コマンド生成不要
- プログラマティックな制御
- エラーハンドリングが容易
- ネストした実行でも環境を引き継げる

## 中間表現（IR）: ContainerConfig

すべての実行要求を統一的に扱うためのデータ構造。

- **基本属性**: イメージ名、コマンド、引数。
- **実行制御**: TTY、インタラクティブモード、自動削除フラグ。
- **環境構成**: ネットワーク設定、ボリュームマウント（Host/Container）、環境変数、作業ディレクトリ、実行ユーザー。

## CRIインターフェース: ContainerRuntime

各ランタイムの差異を吸収するための共通インターフェース。

- **ライフサイクル**: 作成、起動、待機、削除の各フェーズをメソッド化。
- **IO制御**: 標準入出力（stdin/stdout/stderr）のアタッチ。
- **操作**: シグナル送信（SignalContainer）、TTYリサイズ（ResizeContainerTTY）。

### ランタイム実装のポイント

- **Docker実装**: Docker Engine API (`github.com/docker/docker/client`) を使用。
- **Podman実装**: Podman API (`github.com/containers/podman/v4/pkg/bindings`) を使用予定（現在はスタブ）。
- **共通ロジック**: `ContainerConfig` を各ランタイム固有の `Config`, `HostConfig` 等に変換。

## 実行フロー

### 基本的な実行手順

1. **コンテナ作成**: `CreateContainer` で設定を渡し、IDを取得。
2. **クリーンアップ予約**: `config.Remove` が真なら、終了時に `RemoveContainer` を呼ぶよう `defer` 等で設定。
3. **コンテナ起動**: `StartContainer` を呼び出す。
4. **IOアタッチ**: `TTY` または `Interactive` の場合、`AttachContainer` で入出力を接続。
5. **シグナル/リサイズ処理**: 実行中にシグナル転送やTTYリサイズ同期を行う。
6. **終了待機**: `WaitContainer` でプロセス終了を待ち、終了コードを取得。

## ネストした実行の解決

### ネストした実行の解決

CRIを直接使うことで、コンテナ内からcderunを実行しても、同じランタイムインスタンスを使用できる。

- **ランタイム共有**: ホストからマウントされたソケット経由で、コンテナ内からもホストのランタイムを操作。
- **環境変数の引き継ぎ**: 実行ホスト（コンテナ内）の環境変数を `ContainerConfig` にマウントまたは追加することで、ネストしたコンテナに引き継ぐ。

## ロードマップ

### Phase 1: コア機能 (Completed)
- 中間表現（ContainerConfig）の定義
- Docker CRI実装
- 基本的な実行フロー

### Phase 2: 設定管理 (Completed)
- 設定ファイル読み込み
- イメージマッピング
- 優先順位解決
- ドライランモード (Phase 4から前倒しで完了)

### Phase 3: 高度な機能 (Completed)
- 環境変数パススルー
- ソケット・バイナリマウント・ツールマウント

### Phase 4: 利便性向上 (In Progress)
- Podman CRI実装 (Planned)
- エラーハンドリングの強化
- シグナル転送・リサイズ同期 (Completed)
- 詳細ログ機能 (Completed)

## 依存ライブラリ

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
)
```

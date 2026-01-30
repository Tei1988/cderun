# Feature: Direct Container Execution (Phase 1, 2 Completed)

## 概要

各ランタイムの独自APIを介して直接コンテナを実行する。
コマンド生成は行わず、中間表現（IR）を各ランタイムのAPIコールに変換する。

## アーキテクチャ

```
cderunフラグ → 中間表現（IR） → ランタイムAPIコール → コンテナ実行
                    ↓
               ContainerConfig
                    ↓
          runtime.CreateContainer()  ← Docker Engine API
          runtime.StartContainer()   ← Podman API
          runtime.AttachContainer()  ← containerd API
```

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

### ランタイム実装のポイント

- **Docker実装**: Docker Engine API (`github.com/docker/docker/client`) を使用。
- **Podman実装**: Podman API (`github.com/containers/podman/v4/pkg/bindings`) を使用（Docker互換）。
- **共通ロジック**: `ContainerConfig` を各ランタイム固有の `Config`, `HostConfig` 等に変換。

## 実行フロー

### 基本的な実行手順

1. **コンテナ作成**: `CreateContainer` で設定を渡し、IDを取得。
2. **クリーンアップ予約**: `config.Remove` が真なら、終了時に `RemoveContainer` を呼ぶよう `defer` 等で設定。
3. **コンテナ起動**: `StartContainer` を呼び出す。
4. **IOアタッチ**: `TTY` または `Interactive` の場合、`AttachContainer` で入出力を接続。
5. **終了待機**: `WaitContainer` でプロセス終了を待ち、終了コードを取得。

## ネストした実行の解決

### ネストした実行の解決

CRIを直接使うことで、コンテナ内からcderunを実行しても、同じランタイムインスタンスを使用できる。

- **ランタイム共有**: ホストからマウントされたソケット経由で、コンテナ内からもホストのランタイムを操作。
- **環境変数の引き継ぎ**: 実行ホスト（コンテナ内）の環境変数を `ContainerConfig` にマウントまたは追加することで、ネストしたコンテナに引き継ぐ。

## 設定

### ランタイムの選択 (`.cderun.yaml`)

```yaml
runtime: docker  # docker | podman
runtimePath: /usr/bin/docker

defaults:
  tty: false
  interactive: false
```

### ツール設定 (`.tools.yaml`)

```yaml
python:
  image: python:3.11-slim
    tty: true
    interactive: true
    volumes:
      - .:/workspace
    env:
      - PYTHONUNBUFFERED=1
    workdir: /workspace
```

## メリット

### 1. コマンド生成不要
文字列ベースのコマンド組み立てを排除し、構造化された設定を直接APIに渡す。

### 2. 精度の高いエラーハンドリング
ランタイムが返す詳細なエラー情報をそのまま処理可能。

### 3. 環境の引き継ぎ
親プロセスの環境変数などをプログラム的に制御し、新しいコンテナに注入。

### 4. プログラマティックな制御
非同期での状態監視や、複雑なライフサイクル管理が可能。

## ロードマップ

### Phase 1: コア機能 (Completed)
- 中間表現（ContainerConfig）の定義
- Docker CRI実装
- 基本的な実行フロー

### Phase 2: 設定管理 (Completed)
- 設定ファイル読み込み
- イメージマッピング
- 優先順位解決
- ドライランモード

### Phase 3: 高度な機能 (Completed)
- 環境変数の引き継ぎ
- ソケット・バイナリマウント

### Phase 4: 利便性向上 (Planned)
- Podman CRI実装
- エラーハンドリングの強化

## 依存ライブラリ

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
)
```

# ライブラリと技術スタック

このプロジェクトで使用する技術スタックとライブラリの選定基準です。
新しい外部ライブラリを導入する際は、必ずユーザーの許可を得てから `go get` してください。

## 1. 主要技術 (Core Technology)

- **言語:** Go (最新の安定版)
- **モジュール管理:** Go Modules (`go.mod`)

## 2. 選定基準

ライブラリ選定に迷った際は、以下の優先順位で判断してください。

1. **Go標準ライブラリ:** 標準パッケージで実現可能か？（依存関係を減らすため）
1. **シンプルさ:** 機能に対してライブラリが過剰（Overkill）ではないか？
1. **コミュニティ:** GitHubのスター数、メンテナンス頻度、ドキュメントの質は十分か？

## 3. 承認済みライブラリ

現在プロジェクトで使用が承認されている主要ライブラリ：

- **CLIフレームワーク:** [cobra](https://github.com/spf13/cobra)
- **コンテナランタイムAPI:** [moby (Docker)](https://github.com/moby/moby), [containerd/errdefs](https://github.com/containerd/errdefs)
- **YAML & 設定ユーティリティ:** [yaml.v3](https://gopkg.in/yaml.v3), [mergo](https://dario.cat/mergo)
- **ユーティリティ:** [uuid](https://github.com/google/uuid), [go-units](https://github.com/docker/go-units), [x/term](https://golang.org/x/term)
- **テスト:** [testify](https://github.com/stretchr/testify)

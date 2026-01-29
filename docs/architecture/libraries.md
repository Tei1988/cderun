# Libraries & Tech Stack
このプロジェクトで使用する技術スタックとライブラリの選定基準です。
新しい外部ライブラリを導入する際は、必ずユーザーの許可を得てから `go get` してください。

## 1. Core Technology
- **Language:** Go (Latest stable version)
- **Module Management:** Go Modules (`go.mod`)

## 2. Selection Criteria
ライブラリ選定に迷った際は、以下の優先順位で判断してください。

1. **Go Standard Library:** 標準パッケージで実現可能か？（依存関係を減らすため）
1. **Simplicity:** 機能に対してライブラリが過剰（Overkill）ではないか？
1. **Community:** GitHubのスター数、メンテナンス頻度、ドキュメントの質は十分か？

## 3. Approved Libraries
現在プロジェクトで使用が承認されている主要ライブラリ：

- **CLI Framework:** [cobra](https://github.com/spf13/cobra)
- **Container Runtime API:** [moby (Docker)](https://github.com/moby/moby)
- **YAML Parsing:** [yaml.v3](https://gopkg.in/yaml.v3)
- **Testing:** [testify](https://github.com/stretchr/testify)

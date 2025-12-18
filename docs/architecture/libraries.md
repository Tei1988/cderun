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

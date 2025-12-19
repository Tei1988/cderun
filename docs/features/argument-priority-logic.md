# Feature: Argument Priority Logic (P1-P5)

## 概要
設定値は以下の優先順位で解決される。

1. **P1: Override Flags** (`--cderun-tty` 等) - 最優先・強制上書き
2. **P2: Standard Flags** (`--tty` 等) - CLI標準引数
3. **P3: Environment Variables** (`CDERUN_TTY` 等)
4. **P4: Config File** (YAML/JSON)
5. **P5: Hardcoded Defaults**

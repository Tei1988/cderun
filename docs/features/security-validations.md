# Security Validations

cderun implements several validation layers to prevent command injection, path traversal, and log injection attacks.

## Character Validation

The `validatePathChars` function ensures that critical configuration strings do not contain ASCII control characters (ASCII < 32 or 127).

This validation is applied to:

- **Image names**
- **User names** (`user`)
- **Network modes** (`network`)
- **Hostnames** (`hostname`)
- **Working directories** (`workdir`)
- **Runtime names** (`runtime`)
- **Socket paths** (`--socket-path`, `--mount-socket-path`)
- **cderun binary mount path** (`--mount-cderun-path`)
- **Output formats** (`--dry-run-format`, `--diagnosis-format`)
- **Logging settings** (`--log-level`, `--log-format`)
- **Entrypoint elements**
- **Port mappings and exposed ports** (`--publish`, `--expose`)
- **DNS servers and host mappings** (`--dns`, `--add-host`)
- **Linux capabilities** (`--cap-add`, `--cap-drop`)
- **Mount sources and targets** (`--mount`)
- **Device host and container paths** (`--device`)
- **Environment variable keys** (`--env`)

Any string containing unsafe characters will cause an immediate resolution failure.

## Anchor Boundary Validation (アンカー境界検証)

マジックワード（`{{HOME}}`, `{{PWD}}` 等）やチルダ（`~`）を起点としたパス解決において、解決後のパスが起点ディレクトリの境界を越えて親ディレクトリへ遡っていないかを検証します。これにより、ユーザー入力による意図しないパスへのアクセスやディレクトリトラバーサル攻撃を防止します。詳細は [値の解決](./value-resolution.md#アンカー境界検証-anchor-boundary-validation) を参照してください。

## Tool Name Safety

The `ValidateToolName` function enforces strict naming conventions for tool identifiers. Tool names are restricted to a whitelist of safe characters:

- Alphanumerics (`a-z`, `A-Z`, `0-9`)
- Dots (`.`)
- Underscores (`_`)
- Hyphens (`-`)

This prevents tool names from being used for path traversal (e.g., `../../etc/shadow`) or containing shell-sensitive characters (e.g., `|`, `;`, `:`). Tool name validation is performed before any logging or filesystem operations.

## Signal Validation

When signaling containers via `SignalContainer`, signal names are validated against a strict regular expression:
`^(?i)(SIG[A-Z0-9]+|[A-Z0-9]+|[0-9]+)$`

This allows:

- Standard signal names with or without the SIG prefix (e.g., `TERM`, `SIGKILL`)
- Numeric signal values (e.g., `9`, `15`)

Invalid signals are rejected to prevent command injection into the underlying runtime.

## Registry Mismatch Validation

誤ったレジストリや許可されていないレジストリの使用を防止するため、CLIや環境変数で指定されたイメージが、ツールの設定（`.tools.yaml`）で定義されたレジストリと一致するかを検証します。

一致の判定は、ホスト名とリポジトリ名（例: `docker.io/library/node`）に基づいて行われ、タグやダイジェストの違いは許容されます。不一致（例: 設定では `docker.io` を期待しているが CLI で `private-reg.com` が指定された場合）が検出されると、`RegistryMismatchError` を返し、実行を中断します。

## Absolute Mount Targets

All mount configurations must specify an absolute path for the `Target` (container-side path). Relative paths in mount targets are ambiguous and potentially dangerous, so they are rejected during resolution.

## Environment Variable Validation

When validating environment variables, `cderun` applies character checks strictly to the **key** portion (the part before the first `=`). This prevents the use of control characters in variable names while allowing legitimate multiline values or complex strings (such as PEM certificates) in the values themselves.

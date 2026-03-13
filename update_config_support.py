import sys

with open('docs/features/configuration-file-support.md', 'r') as f:
    content = f.read()

# Remove config and toolConfig from the option list around line 110-116
search_text = """- `config` (string): cderun設定ファイルパス。ネスト実行時に子コンテナへ伝播される
- `toolConfig` (string): ツール設定ファイルパス。ネスト実行時に子コンテナへ伝播される"""

# Replace with explanation
replace_text = """> [!IMPORTANT]
> `config` および `toolConfig` は、ネストされた実行時に内部メタデータとして使用されるフィールドであり、ユーザーが設定ファイル（`.cderun.yaml` / `.tools.yaml`）に記述するためのものではありません。
> `cderun` は `internal/command/root.go` の `loadConfigs` メソッドにより、フラグ（`--config`, `--tool-config`）または環境変数（`CDERUN_CONFIG`, `CDERUN_TOOL_CONFIG`）から読み込み先パスを決定します。"""

if search_text in content:
    content = content.replace(search_text, replace_text)
else:
    print("Search text not found in docs/features/configuration-file-support.md")

with open('docs/features/configuration-file-support.md', 'w') as f:
    f.write(content)

import sys

with open('docs/features/configuration-file-support.md', 'r') as f:
    lines = f.readlines()

new_lines = []
skip_mode = False
for line in lines:
    if '- `config` (string): cderun設定ファイルパス。ネスト実行時に子コンテナへ伝播される' in line:
        continue
    if '- `toolConfig` (string): ツール設定ファイルパス。ネスト実行時に子コンテナへ伝播される' in line:
        continue
    if 'config: ""' in line:
        continue
    if 'toolConfig: ""' in line:
        continue
    new_lines.append(line)

content = "".join(new_lines)

# Ensure the note is placed correctly if it was appended at the end or multiple times
# The previous script might have appended it incorrectly.
note = """> [!IMPORTANT]
> `config` および `toolConfig` は、ネストされた実行時に内部メタデータとして使用されるフィールドであり、ユーザーが設定ファイル（`.cderun.yaml` / `.tools.yaml`）に記述するためのものではありません。
> `cderun` は `internal/command/root.go` の `loadConfigs` メソッドにより、フラグ（`--config`, `--tool-config`）または環境変数（`CDERUN_CONFIG`, `CDERUN_TOOL_CONFIG`）から読み込み先パスを決定します。"""

# Clean up existing duplicate notes
content = content.replace(note, "")

# Insert the note before "## 優先順位"
if "## 優先順位" in content:
    content = content.replace("## 優先順位", note + "\n\n## 優先順位")
else:
    content += "\n\n" + note

with open('docs/features/configuration-file-support.md', 'w') as f:
    f.write(content)

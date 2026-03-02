import sys

file_path = "docs/features/INDEX.md"
with open(file_path, "r") as f:
    lines = f.readlines()

new_lines = []
inserted = False
for line in lines:
    if "### 開発・検証機能" in line and not inserted:
        # Insert before 開発・検証機能
        new_lines.append("### 管理・デバッグ機能\n\n")
        new_lines.append("1. **[バージョン管理 (完了)](./version-management.md)**\n\n")
        new_lines.append("  - Git 情報の動的注入 (Tag, SHA, BuildDate)\n")
        new_lines.append("  - 詳細な `--version` 出力\n\n")
        inserted = True
    new_lines.append(line)

with open(file_path, "w") as f:
    f.writelines(new_lines)

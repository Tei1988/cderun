import sys

with open('docs/features/hang-timeout.md', 'r') as f:
    lines = f.readlines()

new_lines = []
for line in lines:
    if 'デフォルトでは2秒（`CDERUN_HANG_TIMEOUT` 環境変数で上書き可能）' in line:
        line = line.replace('デフォルトでは2秒（`CDERUN_HANG_TIMEOUT` 環境変数で上書き可能）',
                            'デフォルトでは2秒（P1: `--cderun-hang-timeout`, P2: `--hang-timeout`, P3: `CDERUN_HANG_TIMEOUT` 等で上書き可能）')
    new_lines.append(line)

with open('docs/features/hang-timeout.md', 'w') as f:
    f.writelines(new_lines)

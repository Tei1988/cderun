import sys

with open('docs/features/hang-timeout.md', 'r') as f:
    content = f.read()

old_text = 'デフォルトでは2秒（P1: `--cderun-hang-timeout`, P2: `--hang-timeout`, P3: `CDERUN_HANG_TIMEOUT` 等で上書き可能）'
new_text = 'デフォルトでは2秒（P1: `--cderun-hang-timeout`, P2: `--hang-timeout`, P3: `CDERUN_HANG_TIMEOUT`, P4: `.tools.yaml` の `hangTimeout`, P5: `.cderun.yaml` の `hangTimeout` で上書き可能）'

content = content.replace(old_text, new_text)

with open('docs/features/hang-timeout.md', 'w') as f:
    f.write(content)

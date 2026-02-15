import sys
import os

def fix_file(path):
    with open(path, 'r') as f:
        lines = f.readlines()

    try:
        start = lines.index('import (\n')
        end = lines.index(')\n', start)
    except ValueError:
        return

    import_lines = lines[start+1:end]
    stdlib = []
    external = []
    local = []

    current_block = []
    for line in import_lines:
        if line.strip() == "":
            continue

        # Determine which group it belongs to
        clean_line = line.strip().strip('"')
        if clean_line.startswith('cderun/'):
            local.append(line)
        elif '.' in clean_line.split('/')[0]:
            external.append(line)
        else:
            stdlib.append(line)

    stdlib.sort()
    external.sort()
    local.sort()

    new_imports = []
    if stdlib:
        new_imports.extend(stdlib)
    if external:
        if new_imports: new_imports.append('\n')
        new_imports.extend(external)
    if local:
        if new_imports: new_imports.append('\n')
        new_imports.extend(local)

    new_lines = lines[:start+1] + new_imports + lines[end:]
    with open(path, 'w') as f:
        f.writelines(new_lines)

files = [
    'internal/command/root.go',
    'internal/command/root_test.go',
    'internal/command/snapshot.go',
    'internal/command/snapshot_test.go',
    'internal/command/robustness_test.go',
    'internal/command/stdin_test.go',
    'internal/command/polyglot_test.go',
    'internal/command/flags_test.go',
    'internal/command/integration_test.go',
    'internal/command/scenario_nested_test.go',
    'internal/command/signals_test.go',
    'internal/command/e2e_device_test.go',
    'internal/command/signals_unix.go',
]

for f in files:
    if os.path.exists(f):
        fix_file(f)

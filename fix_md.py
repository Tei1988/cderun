import os
import re

def fix_markdown(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    # 1. Add 'text' to opening fences that lack an identifier (MD040)
    # 2. Ensure blank lines around fences (MD031)

    lines = content.split('\n')
    new_lines = []
    in_code_block = False

    for i, line in enumerate(lines):
        stripped = line.strip()
        if stripped.startswith('```'):
            if not in_code_block:
                # Opening fence
                # Ensure blank line before
                if len(new_lines) > 0 and new_lines[-1].strip() != "":
                    new_lines.append("")

                if stripped == '```':
                    new_lines.append('```text')
                else:
                    new_lines.append(line)
                in_code_block = True
            else:
                # Closing fence
                new_lines.append('```')
                # Ensure blank line after if not last line
                if i < len(lines) - 1 and lines[i+1].strip() != "":
                    new_lines.append("")
                in_code_block = False
        else:
            new_lines.append(line)

    # Final cleanup of extra newlines
    final_content = '\n'.join(new_lines)
    # Standardize to at most one blank line between paragraphs
    final_content = re.sub(r'\n{3,}', r'\n\n', final_content)

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(final_content.strip() + '\n')

targets = ['README.md']
docs_dir = 'docs/features'
for f in os.listdir(docs_dir):
    if f.endswith('.md'):
        targets.append(os.path.join(docs_dir, f))

for t in targets:
    fix_markdown(t)

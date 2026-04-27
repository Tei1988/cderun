import os
import re

filepath = 'internal/config/path_security_test.go'
with open(filepath, 'r') as f:
    lines = f.readlines()

new_lines = []
for line in lines:
    if '{name: "No anchor no traversal check", input: "../../etc/passwd"},' in line:
        # Relative paths are resolved against baseDir, not restricted by default unless anchor is used
        new_lines.append(line)
        # Add a test case for find_dir absolute path
        new_lines.append('\t\t{name: "find_dir returns absolute path", input: "{{find_dir:.git}}/file", extraDirs: map[string]bool{"/work/.git": true}},\n')
    elif '{name: "Safe find_dir anchor", input: "{{find_dir:.git}}/file", extraDirs: map[string]bool{"/work/.git": true}},' in line:
        # This is already what I want to add, but it's already there.
        # The issue is it might fail if it's not absolute in the anchor validation.
        new_lines.append(line)
    else:
        new_lines.append(line)

with open(filepath, 'w') as f:
    f.writelines(new_lines)

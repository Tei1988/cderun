import sys

with open('internal/config/path.go', 'r') as f:
    content = f.read()

# Replace the regex line with the one that matches optional leading slash at the start
lines = content.split('\n')
for i, line in enumerate(lines):
    if 'magicWordPreRegex = regexp.MustCompile' in line:
        lines[i] = '\tmagicWordPreRegex = regexp.MustCompile(`^(/?)({{\\s*(HOME|PWD|BASE_HOME|BASE_PWD)\\s*}}|~)` )'.replace(' )', ')')
        break

content = '\n'.join(lines)

# The indices anchor := matches[2] and word := matches[3] should be correct for ^(/?)(...)
# matches[0] = full match
# matches[1] = optional slash
# matches[2] = anchor
# matches[3] = word (if magic word)

with open('internal/config/path.go', 'w') as f:
    f.write(content)

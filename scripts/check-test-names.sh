#!/usr/bin/env bash
set -euo pipefail

# Prohibited words pattern for test file naming
PROHIBITED_PATTERN='(improvement|expansion|refinement|comprehensive|additional|extra|more|deep|jules)'

# Determine target files to check:
# 1. If base branch / target ref is provided or can be determined (e.g. against main/origin/main), check modified/added test files.
# 2. Otherwise, check uncommitted modified/staged/untracked test files.

BASE_REF=""
if [ -n "${TARGET_BRANCH:-}" ] && [ "${TARGET_BRANCH}" != "0000000000000000000000000000000000000000" ] && git rev-parse --verify "${TARGET_BRANCH}" >/dev/null 2>&1; then
  BASE_REF="${TARGET_BRANCH}"
fi

if [ -z "${BASE_REF}" ]; then
  if git rev-parse --verify origin/main >/dev/null 2>&1; then
    BASE_REF="origin/main"
  elif git rev-parse --verify main >/dev/null 2>&1; then
    BASE_REF="main"
  fi
fi

FILES=""
if [ -n "${BASE_REF}" ] && git rev-parse --verify "${BASE_REF}" >/dev/null 2>&1; then
  FILES=$(git diff --name-only --diff-filter=ACMRT "${BASE_REF}...HEAD" -- '*_test.go' 2>/dev/null || true)
fi

# Fallback or additional check for working tree changes (staged and unstaged, handling renames R old -> new)
WORKTREE_FILES=$(git status --porcelain -- '*_test.go' 2>/dev/null | awk '{print ($NF)}' || true)
ALL_TARGET_FILES=$(echo -e "${FILES}\n${WORKTREE_FILES}" | grep -v '^$' | sort -u || true)

if [ -z "${ALL_TARGET_FILES}" ]; then
  echo "No modified test files to verify."
  exit 0
fi

VIOLATIONS=""
for file in ${ALL_TARGET_FILES}; do
  filename=$(basename "$file")
  if echo "$filename" | grep -E -q "${PROHIBITED_PATTERN}"; then
    VIOLATIONS="${VIOLATIONS}\n - ${file}"
  fi
done

if [ -n "${VIOLATIONS}" ]; then
  echo "Error: Test file naming policy violation detected in modified/added test files!"
  echo "Test file names must describe 'what is being tested' and must NOT contain prohibited words (improvement|expansion|refinement|comprehensive|additional|extra|more|deep|jules)."
  echo -e "Violating files:${VIOLATIONS}"
  exit 1
fi

echo "Test file naming check passed successfully."

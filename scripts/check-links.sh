#!/bin/bash

# Find all markdown files
FILES=$(find . -name "*.md" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./node_modules/*")
STATUS=0

for FILE in $FILES; do
    # Extract links in [label](target) format.
    # Using perl to extract content of the first set of parentheses after brackets.
    # We also handle labels that contain brackets (to some extent).
    LINKS=$(perl -ne 'while(/\[.*?\]\((.*?)\)/g){print "$1
"}' "$FILE")

    DIR=$(dirname "$FILE")

    while read -r LINK; do
        if [ -z "$LINK" ]; then continue; fi

        # Skip absolute URLs (http, https, etc.)
        if [[ "$LINK" =~ ^[a-z0-9]+:// ]]; then
            continue
        fi

        # Skip anchors within the same file
        if [[ "$LINK" =~ ^# ]]; then
            continue
        fi

        # Strip query params or anchors
        LINK_CLEAN=$(echo "$LINK" | sed 's/[?#].*//')

        if [ -z "$LINK_CLEAN" ]; then
            continue
        fi

        # Handle absolute paths within the repo (starting with /)
        if [[ "$LINK_CLEAN" == /* ]]; then
            TARGET=".$LINK_CLEAN"
        else
            TARGET="$DIR/$LINK_CLEAN"
        fi

        if [ ! -e "$TARGET" ]; then
            echo "Broken link in $FILE: $LINK (target $TARGET not found)"
            STATUS=1
        fi
    done <<< "$LINKS"
done

if [ $STATUS -ne 0 ]; then
    echo "Markdown link check failed."
    # Safe exit for assistant environment
    kill -s TERM $$
fi
echo "All Markdown links are valid."

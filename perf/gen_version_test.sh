#!/bin/bash

VERSION_FILE="perf/VERSION.txt"

# Function to exit with an error message
fail() {
  echo "ERROR: $1" >&2
  exit 1
}

# Check if the file exists
if [[ ! -f "$VERSION_FILE" ]]; then
  fail "VERSION.txt not found!"
fi

# Check if the file is not empty
if [[ ! -s "$VERSION_FILE" ]]; then
  fail "VERSION.txt is empty!"
fi

# Check if the content looks like a git hash (40 hex chars) or "unknown" / "unversioned"
content=$(cat "$VERSION_FILE")
if [[ ! "$content" =~ ^[0-9a-f]{40}$ && \
     "$content" != "unknown" && \
     "$content" != "unversioned" ]]; then
  msg="VERSION.txt content '$content'"
  msg+=" doesn't look like a git hash or 'unknown' or 'unversioned'!"
  fail "$msg"
fi

echo "PASS"
exit 0
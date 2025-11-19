#!/bin/bash
# Helper script to run linters for specific languages.
# Usage: ./run_lint.sh {py|sh}

set -e

case "$1" in
  py)
    echo "Running Python linter (pylint via Bazel)..."
    # We use $(pwd) to get absolute paths because bazel run changes the working directory.
    bazelisk run --config=mayberemote //:pylint -- --rcfile="$(pwd)/.pylintrc" \
      $(find "$(pwd)" -name '*.py' -not -path '*/node_modules/*' -not -path '*/_bazel_*/*')
    ;;
  sh)
    echo "Running Shell linter (shellcheck)..."
    # Ensure shellcheck from node_modules is found
    export PATH="$(pwd)/node_modules/.bin:$PATH"
    find . -type f -name "*.sh" -not -path "*/node_modules/*" -not -path "*/_bazel_*/*" \
      | xargs shellcheck
    ;;
  *)
    echo "Usage: $0 {py|sh}"
    exit 1
    ;;
esac

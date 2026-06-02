#!/bin/bash
#
# hot-reload.sh: Automatically rebuilds the Perf frontend and refreshes the browser.
#
# This script watches the `perf/modules` directory for changes. When a file is modified,
# it runs the Bazel build command for the frontend and signals the running DevMode
# server to either hot-swap styles (for .css/.scss) or perform a full page reload.
#
# Usage:
#   ./hot-reload.sh           # Runs using 'entr' (recommended, requires `sudo apt install entr`)
#   ./hot-reload.sh --poll    # Runs using a polling loop (useful for network mounts or Cider-G)
#   ./hot-reload.sh --nopoll  # Forces 'entr' mode even if not in a Git workspace
#
# Note: The script automatically falls back to polling mode if run outside a
# standard Git workspace (e.g., in a Cider-G citc client).

# Ensure we run from the perf directory
cd "$(dirname "$0")" || exit 1

WATCH_DIR="modules"
POLL_INTERVAL=0.3
export BUILD_CMD="bazelisk build --config=mayberemote -c dbg //perf/pages:dev_pages"
export TRIGGER_URL="http://localhost:8002/__trigger_reload"

handle_change() {
  CHANGED_FILE=$1
  echo "Detected change in: $CHANGED_FILE"

  if $BUILD_CMD; then
    if [[ "$CHANGED_FILE" == *.css ]] || [[ "$CHANGED_FILE" == *.scss ]]; then
      echo "CSS change. Triggering style swap..."
      curl -s -X POST "$TRIGGER_URL?type=css" > /dev/null
    else
      echo "Core change. Triggering full reload..."
      curl -s -X POST "$TRIGGER_URL?type=full" > /dev/null
    fi
  else
    echo "Build failed. Holding reload."
  fi
}

export -f handle_change

FORCE_POLL=false
FORCE_NOPOLL=false

for arg in "$@"; do
  if [[ "$arg" == "--poll" || "$arg" == "-p" ]]; then
    FORCE_POLL=true
  elif [[ "$arg" == "--nopoll" ]]; then
    FORCE_NOPOLL=true
  fi
done

USE_POLLING=false

if [ "$FORCE_NOPOLL" = true ]; then
  USE_POLLING=false
elif [ "$FORCE_POLL" = true ]; then
  USE_POLLING=true
else
  # Auto-detect: if not in a git repo, default to polling
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Not a git workspace (likely Cider-G/citc). Defaulting to polling mode..."
    USE_POLLING=true
  fi
fi

if [ "$USE_POLLING" = true ]; then
  echo "Polling $WATCH_DIR for changes (Network/VM Mode)..."

  REF_FILE="/tmp/.perf_watch_ref_$$"
  trap 'rm -f "$REF_FILE"' EXIT
  touch "$REF_FILE"

  while true; do
    CHANGED_FILES=$(find "$WATCH_DIR" -type f -newer "$REF_FILE" 2>/dev/null)

    if [[ -n "$CHANGED_FILES" ]]; then
      touch "$REF_FILE"
      for FILE in $CHANGED_FILES; do
        handle_change "$FILE"
        # In polling mode we just run one build no matter how many files changed.
        break
      done
    fi
    sleep "$POLL_INTERVAL"
  done

else
  echo "Watching $WATCH_DIR for changes using entr..."

  if ! command -v entr &> /dev/null; then
      echo "Error: 'entr' is not installed."
      echo "Please install it (e.g., 'sudo apt install entr'), or run this script with the --poll flag to bypass it."
      exit 1
  fi

  while true; do
    find "$WATCH_DIR" -type f | entr -c -d bash -c 'handle_change "/_"'
  done
fi

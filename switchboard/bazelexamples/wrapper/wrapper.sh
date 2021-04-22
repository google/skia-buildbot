#!/bin/sh

# This script wraps a test binary (passed in via --test-bin), and shows how one might run arbitrary
# command before and after executing the wrapped test.

log() {
  echo "[wrapper script] $1"
}

# Check that --test-bin is set.
if [[ -z $1 || $1 != "--test-bin" || -z $2 ]]; then
  echo "Usage: $0 --test-bin <path>"
  exit 1
fi

# Read the --test-bin flag.
TEST_BIN="$2"

# Set the environment variable expected by the wrapped test, which must point to a temporary file.
export WRAPPER_SCRIPT_TEMP_FILE=$(mktemp "/tmp/wrapper_script_temp_file.XXXXXXXXXX")
echo "Hello, world!" > $WRAPPER_SCRIPT_TEMP_FILE

# Run the wrapped test binary.
log "Running test binary: ${TEST_BIN}"
"$(realpath $TEST_BIN)"
test_exit_code=$?  # Save the test exit code.
log "Test binary exited with code ${test_exit_code}."

# Clean up.
rm $WRAPPER_SCRIPT_TEMP_FILE

# Forward the exit code to Bazel. Bazel will report test failure on non-zero exit codes.
exit $test_exit_code

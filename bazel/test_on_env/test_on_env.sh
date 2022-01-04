#!/bin/bash
#
# Runner script for the test_on_env Bazel build rule.

# These will be populated/overwritten from command-line flags.
TEST_BIN=
ENV_BIN=
READY_CHECK_TIMEOUT=

printUsageAndDie() {
  echo "Usage: test_on_env.sh <test_bin> <env_bin> <ready_check>"
  echo ""
  echo "Required flags:"
  echo " test_bin: path to the test binary"
  echo " env_bin: path to the environment binary"
  echo " ready_check: wait up to <s> seconds for the environment to be ready"
  exit 1
}

parseFlags() {
  TEST_BIN=$1
  ENV_BIN=$2
  READY_CHECK_TIMEOUT=$3
  # Validate required flags.
  if [[ -z "$TEST_BIN" || -z "$ENV_BIN" || -z "$READY_CHECK_TIMEOUT" ]]; then
    printUsageAndDie
  fi
}

log() {
  echo "[test_on_env] $1"
}

main() {
  parseFlags $@

  # Set shared environment variables. Both the environment and the test binaries will see this.
  export ENV_DIR=$TEST_TMPDIR/envdir
  export ENV_READY_FILE=$ENV_DIR/ready

  mkdir $ENV_DIR

  log "Starting test_on_env test..."
  log "  Path to environment binary  = $ENV_BIN"
  log "  Path to test binary         = $TEST_BIN"
  log "  Ready check timeout (secs)  = $READY_CHECK_TIMEOUT"
  log "  TEST_TMPDIR                 = $TEST_TMPDIR"
  log "  ENV_DIR                     = $ENV_DIR"
  log "  ENV_READY_FILE              = $ENV_READY_FILE"

  # Start the environment.
  $ENV_BIN &
  local env_pid=$!
  log "Environment started with PID $env_pid."

  # Wait for the environment to be ready.
  log "Waiting up to $READY_CHECK_TIMEOUT seconds for environment to be ready..."
  local seconds_counter=1
  until [[ -f "$ENV_READY_FILE" ]]; do
    if [[ $seconds_counter -gt $READY_CHECK_TIMEOUT ]]; then
        log "Timed out while waiting for environment to be ready."
        kill -s SIGTERM $env_pid  # Tear down environment.
        exit 1  # Return a non-zero exit code in order for Bazel to report test failure.
    fi
    sleep 1
    let "seconds_counter += 1"
  done

  # Run tests.
  #
  # For some unknown reason, Go tests fail with "fork/exec [...]: no such file or directory" when
  # invoked from this script via $TEST_BIN, which holds the path to a symlink created by Bazel.
  # Invoking the test binary via its real path, as opposed to a symlink, prevents this error.
  "$(realpath $TEST_BIN)"
  local test_exit_code=$?
  log "Test exit code: $test_exit_code"

  # Tear down environment.
  kill -s SIGTERM $env_pid

  # Forward the test exit code to Bazel.
  exit $test_exit_code
}

main $@

#!/bin/bash
#
# Runner script for the test_on_env Bazel build rule.

CMDLINE_ARGS=$@

# These will be populated/overwritten from command-line flags.
TEST_BIN=
ENV_BIN=
READY_CHECK_TIMEOUT=5

printUsageAndDie() {
  echo "Usage: $0 <flags>"
  echo ""
  echo "Required flags:"
  echo " -t, --test-bin <path>               path to the test bianry"
  echo " -e, --env-bin <path>                path to the environment binary"
  echo ""
  echo "Optional flags:"
  echo " -r, --ready-check-timeout-secs <s>  wait up to <s> seconds for the environment to be ready"
  exit 1
}

parseFlags() {
  options=$(getopt -u --name $0 \
                   --options t:e:r: \
                   --longoptions test-bin:,env-bin:,ready-check-timeout-secs: \
                   -- ""$CMDLINE_ARGS"")
  if [ $? != "0" ]; then
    printUsageAndDie
  fi
  set -- $options

  while true; do
    case "$1"
    in
      -t|--test-bin)
        TEST_BIN="$2"; shift;;
      -e|--env-bin)
        ENV_BIN="$2"; shift;;
      -r|--ready-check-timeout-secs)
        READY_CHECK_TIMEOUT="$2"; shift;;
      --)
        shift; break;;
      *)
        printUsageAndDie;;
    esac
    shift
  done

  # Validate required flags.
  if [[ -z "$TEST_BIN" || -z "$ENV_BIN" ]]; then
    printUsageAndDie
  fi
}

log() {
  echo "[test_on_env] $1"
}

main() {
  parseFlags

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
  "$TEST_BIN"
  local test_exit_code=$?
  log "Test exit code: $test_exit_code"

  # Tear down environment.
  kill -s SIGTERM $env_pid

  # Forward the test exit code to Bazel.
  exit $test_exit_code
}

main

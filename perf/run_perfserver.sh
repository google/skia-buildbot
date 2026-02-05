#!/bin/bash
# Runs a perf instance against a pre-configured spanner database,
# rebuilding and restarting on code changes.
# Prerequisites:
# 1. gcloud auth application-default login
# 2. bazelisk in PATH

# Example Command:
# ./run_perfserver.sh --instance chrome [--proxy]

if ! command -v bazelisk > /dev/null 2>&1;
then
    echo "Error: bazelisk command not found. Please ensure it's in your PATH."
    exit 1
fi

# --- Dependency Checks ---
MISSING_DEPS=false
for cmd in docker nc fuser; do
  if ! command -v "$cmd" > /dev/null 2>&1;
    then
    echo "Error: Required command '$cmd' not found. Please ensure it's in your PATH."
    MISSING_DEPS=true
  fi
done

if [ "$MISSING_DEPS" = true ]; then
  exit 1
fi

# --- ADC Check ---
ADC_FILE="$HOME/.config/gcloud/application_default_credentials.json"
if [ ! -f "$ADC_FILE" ]; then
  echo "Error: Application Default Credentials file not found at: $ADC_FILE"
  echo "Please run: gcloud auth application-default login"
  exit 1
fi

# --- Docker Permission Check ---
DOCKER_CMD="docker"
if ! docker ps > /dev/null 2>&1; then
    if sudo docker ps > /dev/null 2>&1; then
        DOCKER_CMD="sudo docker"
        echo "Docker requires sudo. Using 'sudo docker'."
    fi
fi

# --- Configuration presets ---
# Common constants to reduce redundancy and line length
CORP="skia-infra-corp"
PUB="skia-infra-public"
SPAN_CORP="tfgen-spanid-20241205020733610"
SPAN_PUB="tfgen-spanid-20241204191122045"
GSRC="chromium.googlesource.com"
ASRC="android.googlesource.com"
TSRC="turquoise-internal.googlesource.com"
SSRC="skia.googlesource.com"

declare -A PROJECTS SPANNER_INSTANCES DATABASES CONFIGS DOMAINS REPOS

# Configuration table
while read -r instance project spanner db config_name domain repo; do
  [[ -z "$instance" || "$instance" =~ ^# ]] && continue
  PROJECTS[$instance]="$project"
  SPANNER_INSTANCES[$instance]="$spanner"
  DATABASES[$instance]="$db"
  CONFIGS[$instance]="perf/configs/spanner/$config_name"
  DOMAINS[$instance]="$domain"
  REPOS[$instance]="$repo"
done <<EOF
chrome   $CORP $SPAN_CORP chrome_int      chrome-internal.json  $GSRC chromium/src
v8       $CORP $SPAN_CORP v8_int          v8-internal.json      $GSRC v8/v8
fuchsia  $CORP $SPAN_CORP fuchsia_int     fuchsia-internal.json $TSRC integration
germanium $CORP $SPAN_CORP germanium_evals germanium-internal.json $GSRC chromium/src
android  $PUB  $SPAN_PUB  androidx        android2.json         $ASRC platform/frameworks/support
angle    $PUB  $SPAN_PUB  angle           angle.json            $GSRC angle/angle
flutter  $PUB  $SPAN_PUB  flutter_flutter flutter-flutter.json  $GSRC flutter/flutter
widevine $CORP $SPAN_CORP widevine_cdm    widevine-cdm.json     $GSRC chromium/src
webrtc   $PUB  $SPAN_PUB  webrtc_pub      webrtc-public.json    $GSRC chromium/src
skia     $PUB  $SPAN_PUB  skia            skia-public.json      $SSRC skia
v8_pub   $PUB  $SPAN_PUB  v8              v8-public.json        $GSRC v8/v8
EOF

VALID_INSTANCES="${!PROJECTS[@]}"

usage() {
  echo "Usage: $0 --instance <instance_type> [--proxy] [--prod] [--breakglass]"
  echo "  Valid instance types are: ${VALID_INSTANCES// / | }"
  echo "  --proxy: Build and run the auth-proxy in the background."
  echo "  --prod: Disable --localToProd flag."
  echo "  --breakglass: Request breakglass access before starting."
  exit 1
}

INSTANCE=""
RUN_PROXY=false
USE_LOCAL_TO_PROD=true
RUN_BREAKGLASS=false
DISABLE_SHORTCUT_UPDATE=false

# Parse arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --instance)
            INSTANCE="$2"
            shift
            ;;
        --proxy)
            RUN_PROXY=true
            ;;
        --prod)
            USE_LOCAL_TO_PROD=false
            ;;
        --breakglass)
            RUN_BREAKGLASS=true
            ;;
        --disable-shortcut|--disable_shortcut_update)
            DISABLE_SHORTCUT_UPDATE=true
            ;;
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown parameter passed: $1"
            usage
            ;;
    esac
    shift
done

if [[ -z "$INSTANCE" ]]; then
  echo "Error: --instance flag is required."
  usage
fi

if [[ ! -v PROJECTS["$INSTANCE"] ]]; then
  echo "Error: Invalid instance type: $INSTANCE"
  usage
fi

PROJECT=${PROJECTS[$INSTANCE]}
SPANNER_INSTANCE=${SPANNER_INSTANCES[$INSTANCE]}
DATABASE=${DATABASES[$INSTANCE]}
CONFIG=${CONFIGS[$INSTANCE]}
DOMAIN=${DOMAINS[$INSTANCE]}
REPO=${REPOS[$INSTANCE]}

export GOOGLE_CLOUD_PROJECT="$PROJECT"

# Validate config file existence
if [ ! -f "$CONFIG" ]; then
  echo "Error: Configuration file not found: $CONFIG"
  exit 1
fi

# Create a temporary config file that disables notifications to avoid
# Secret Manager permission issues when running locally.
TEMP_CONFIG="/tmp/perf_config_${INSTANCE}_$$.json"
sed -e 's/"notifications": ".*"/"notifications": "none"/' \
    -e 's/"notification_type": ".*"/"notification_type": "none"/' \
    -e 's/"type": "redis"/"type": "local"/' \
    "$CONFIG" > "$TEMP_CONFIG"
echo "Created temporary config file: $TEMP_CONFIG (notifications disabled, local cache)"

# --- Dynamic Port Allocation ---
echo "Finding free port slot..."
OFFSET=0
STEP=10
while true; do
    PERF_PORT=$((8002 + OFFSET))
    PERF_PROM_PORT=$((20001 + OFFSET))
    PG_PORT=$((5432 + OFFSET))
    PROXY_PORT=$((8003 + OFFSET))
    PROXY_PROM_PORT=$((20003 + OFFSET))
    INTERNAL_PORT=$((9000 + OFFSET))

    # Check if main ports are in use
    if nc -z 127.0.0.1 $PERF_PORT >/dev/null 2>&1 || \
       nc -z 127.0.0.1 $PERF_PROM_PORT >/dev/null 2>&1 || \
       nc -z 127.0.0.1 $PG_PORT >/dev/null 2>&1 || \
       nc -z 127.0.0.1 $INTERNAL_PORT >/dev/null 2>&1; then
       OFFSET=$((OFFSET + STEP))
       continue
    fi

    # If proxy is requested, check those ports too
    if [ "$RUN_PROXY" = true ]; then
        if nc -z 127.0.0.1 $PROXY_PORT >/dev/null 2>&1 || \
           nc -z 127.0.0.1 $PROXY_PROM_PORT >/dev/null 2>&1; then
           OFFSET=$((OFFSET + STEP))
           continue
        fi
    fi

    # All required ports for this offset are free
    break
done

echo "Selected ports (Offset: $OFFSET):"
echo "  Perfserver: $PERF_PORT (metrics: $PERF_PROM_PORT)"
echo "  Postgres:   $PG_PORT"
if [ "$RUN_PROXY" = true ]; then
    echo "  Auth-Proxy: $PROXY_PORT (metrics: $PROXY_PROM_PORT)"
fi

# Use a hash of the current directory to allow running the same instance from different checkouts
DIR_HASH=$(pwd | md5sum 2>/dev/null | awk '{print $1}')
if [ -z "$DIR_HASH" ]; then
    # Fallback if md5sum is missing
    DIR_HASH=$(pwd | sed 's/[^a-zA-Z0-9]/_/g')
fi
LOCKFILE="/tmp/run_perfserver_${INSTANCE}_${DIR_HASH}.lock"
MARKER_FILE="/tmp/perf_server_marker_${INSTANCE}_${DIR_HASH}"

if [ -e "$LOCKFILE" ]; then
  LOCKED_PID=$(cat "$LOCKFILE")
  if kill -0 "$LOCKED_PID" > /dev/null 2>&1; then
    echo "Error: Lock file exists: $LOCKFILE"
    echo "Another instance of the script for '$INSTANCE' in this directory is running"
    echo "(PID: $LOCKED_PID)."
    rm -f "$TEMP_CONFIG"
    exit 1
  else
    echo "Found stale lock file (PID: $LOCKED_PID). Removing it."
    rm -f "$LOCKFILE"
  fi
fi
echo $$ > "$LOCKFILE"
touch "$MARKER_FILE"

echo "Using the following params: -p=$PROJECT -i=$SPANNER_INSTANCE -d=$DATABASE \
  -config=$TEMP_CONFIG -domain=$DOMAIN -repo=$REPO"

# Include hash in container name to avoid collisions between different directories
# running same instance
CONTAINER_NAME="perf-pgadapter-${INSTANCE}-${DIR_HASH}"
SERVER_PID=0
AUTH_PROXY_PID=0
RETRY_INTERVAL=60
LAST_BUILD_ATTEMPT=0

stop_server() {
  if [ "$SERVER_PID" -ne 0 ]; then
    echo "Stopping old perfserver (PID: $SERVER_PID)..."
    kill "$SERVER_PID" > /dev/null 2>&1
    # Give it a moment to shut down
    sleep 0.5
    wait "$SERVER_PID" > /dev/null 2>&1
    SERVER_PID=0
  fi
}

build_server() {
  echo "Building perfserver..."
  LAST_BUILD_ATTEMPT=$(date +%s)
  bazelisk build //perf/... --config=mayberemote -c dbg
}

start_server() {
  echo "Starting new perfserver in background..."
  LOG_FILE="/tmp/perfserver_${INSTANCE}_$(date +%Y%m%d_%H%M%S).log"
  echo "Log file: $LOG_FILE"

  LOCAL_TO_PROD_FLAG=""
  if [ "$USE_LOCAL_TO_PROD" = true ]; then
      LOCAL_TO_PROD_FLAG="--localToProd"
  fi

  DISABLE_SHORTCUT_UPDATE_FLAG=""
  if [ "$DISABLE_SHORTCUT_UPDATE" = true ]; then
      DISABLE_SHORTCUT_UPDATE_FLAG="--disable_shortcut_update"
  fi

  # Determine bazel output directory
  BAZEL_BIN="_bazel_bin"
  if [ -d "bazel-bin" ]; then
      BAZEL_BIN="bazel-bin"
  fi

  # Verify critical resource existence to avoid crash-on-request
  REQUIRED_RESOURCE="$BAZEL_BIN/perf/pages/development/newindex.html"
  if [ ! -f "$REQUIRED_RESOURCE" ]; then
      echo "Error: Required resource not found: $REQUIRED_RESOURCE"
      echo "The build might be incomplete. Please check 'bazelisk build //perf/...' output."
      return 1
  fi

  $BAZEL_BIN/perf/go/perfserver/perfserver_/perfserver frontend \
      --dev_mode \
      $LOCAL_TO_PROD_FLAG \
      $DISABLE_SHORTCUT_UPDATE_FLAG \
      --do_clustering=false \
      --port=:$PERF_PORT \
      --prom_port=:$PERF_PROM_PORT \
      --internal_port=:$INTERNAL_PORT \
      --config_filename="$TEMP_CONFIG" \
      --display_group_by=false \
      --disable_metrics_update=true \
      --resources_dir=$BAZEL_BIN/perf/pages/development/ \
      --connection_string="postgresql://root@127.0.0.1:$PG_PORT/${DATABASE}?sslmode=disable" \
      --commit_range_url="https://${DOMAIN}/${REPO}/+log/{begin}..{end}" > "$LOG_FILE" 2>&1 &
  SERVER_PID=$!
  echo "Perfserver process started (PID: $SERVER_PID)"

  echo "Waiting for perfserver to be ready on port $PERF_PORT..."
  MAX_TRIES=150
  COUNT=0
  while ! nc -z 127.0.0.1 $PERF_PORT > /dev/null 2>&1; do
    if ! kill -0 $SERVER_PID > /dev/null 2>&1; then
      echo "Perfserver process (PID: $SERVER_PID) died unexpectedly."
      echo -e "\n--- Last 100 lines of perfserver log ---"
      tail -n 100 "$LOG_FILE"
      echo -e "\n--- Last 50 lines of pgadapter log ---"
      $DOCKER_CMD logs "$CONTAINER_NAME" 2>&1 | tail -n 50
      if $DOCKER_CMD logs "$CONTAINER_NAME" 2>&1 | \
         grep -q "PERMISSION_DENIED: Cloud Monitoring API has not been used in project"; then
         echo -e "\n*** DETECTED WRONG PROJECT CONFIGURATION ***"
         echo "Your credentials are likely configured with an incorrect quota project."
         echo "Please run the following command to fix it:"
         echo "  gcloud auth application-default set-quota-project $PROJECT"
         echo "**********************************************"
      fi
      echo "---------------------------------"
      SERVER_PID=0
      return 1
    fi
    sleep 2
    COUNT=$((COUNT + 1))
    if [ $COUNT -ge $MAX_TRIES ]; then
      echo "Perfserver failed to become ready after $(($MAX_TRIES * 2)) seconds."
      echo -e "\n--- Last 100 lines of perfserver log ---"
      tail -n 100 "$LOG_FILE"
      kill "$SERVER_PID" > /dev/null 2>&1
      SERVER_PID=0
      return 1
    fi
    echo -n "."
  done
  HOSTNAME=$(hostname -f)
  echo -e "\nPerfserver is up and running at http://${HOSTNAME}:$PERF_PORT"
  if [ "$RUN_PROXY" = true ]; then
    echo "Auth proxy is up and running at http://${HOSTNAME}:$PROXY_PORT"
  fi
  return 0
}

attempt_build_and_start() {
    if build_server; then
        if start_server; then
            return 0
        else
            echo "Start failed. Will retry in $RETRY_INTERVAL seconds or on file change."
            return 1
        fi
    else
        echo "Build failed. Will retry in $RETRY_INTERVAL seconds or on file change."
        return 1
    fi
}

# Cleanup function to stop docker container and background processes
cleanup() {
  echo -e "\nShutting down..."
  # Disable further traps to prevent re-entry
  trap - INT TERM

  stop_server

  if [ "$AUTH_PROXY_PID" -ne 0 ]; then
    echo "Stopping auth-proxy (PID: $AUTH_PROXY_PID)..."
    kill "$AUTH_PROXY_PID" > /dev/null 2>&1
    wait "$AUTH_PROXY_PID" > /dev/null 2>&1
  fi

  # Failsafe: ensure ports are cleared
  fuser -k $PERF_PORT/tcp >/dev/null 2>&1
  fuser -k $PERF_PROM_PORT/tcp >/dev/null 2>&1
  fuser -k $INTERNAL_PORT/tcp >/dev/null 2>&1
  fuser -k $PG_PORT/tcp >/dev/null 2>&1
  if [ "$RUN_PROXY" = true ]; then
      fuser -k $PROXY_PORT/tcp >/dev/null 2>&1
      fuser -k $PROXY_PROM_PORT/tcp >/dev/null 2>&1
  fi

  if [ -n "$CONTAINER_NAME" ]; then
    echo "Stopping pgadapter container..."
    $DOCKER_CMD rm -f "$CONTAINER_NAME" > /dev/null 2>&1
  fi
  rm -f "$LOCKFILE"
  rm -f "$MARKER_FILE"
  if [ -f "$TEMP_CONFIG" ]; then
    rm -f "$TEMP_CONFIG"
  fi
  echo "Exited"
  exit 0
}

# Trap exit signals to run cleanup
trap cleanup INT TERM

# First delete any existing docker container for this instance to start clean.
echo "Removing any existing docker container for this instance..."
$DOCKER_CMD rm -f "$CONTAINER_NAME" > /dev/null 2>&1

# Now let's run pgadapter connected to the supplied spanner database.
echo "Starting pgadapter on port $PG_PORT..."
$DOCKER_CMD run -d --rm --name "$CONTAINER_NAME" -p 127.0.0.1:$PG_PORT:5432 \
  -e GOOGLE_CLOUD_PROJECT="$PROJECT" \
  -e GOOGLE_CLOUD_QUOTA_PROJECT="$PROJECT" \
  -v "$ADC_FILE":/acct_credentials.json \
  gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
  -p "$PROJECT" -i "$SPANNER_INSTANCE" -d "$DATABASE" \
  -c /acct_credentials.json -x > /dev/null

echo "pgadapter container started: $CONTAINER_NAME"
echo "Waiting for pgadapter to be ready on port $PG_PORT..."
MAX_PG_TRIES=30
PG_COUNT=0
while ! nc -z 127.0.0.1 $PG_PORT > /dev/null 2>&1; do
  sleep 1
  PG_COUNT=$((PG_COUNT + 1))
  if [ $PG_COUNT -ge $MAX_PG_TRIES ]; then
    echo -e "\nError: pgadapter failed to become ready after $MAX_PG_TRIES seconds."
    if ! $DOCKER_CMD ps -q -f name="$CONTAINER_NAME" | grep -q .; then
       echo "pgadapter container died. Docker logs:"
       $DOCKER_CMD logs "$CONTAINER_NAME"
    fi
    exit 1
  fi
  echo -n "."
done
echo -e "\npgadapter is ready! Waiting 5s for full initialization..."
sleep 5

PROJECT_ROOT=$(readlink -f "$(dirname "$0")/..")
cd "$PROJECT_ROOT" || exit 1
echo "Current directory: $(pwd)"

if [ "$RUN_PROXY" = true ]; then
  echo "Building and starting auth proxy..."
  # Using bazelisk directly to avoid hardcoded ports in Makefile
  bazelisk run //kube/cmd/auth-proxy -- \
      --prom-port=:$PROXY_PROM_PORT \
      --role=editor=google.com \
      --authtype=mocked \
      --mock_user=$(whoami)@google.com \
      --port=:$PROXY_PORT \
      --target_port=http://127.0.0.1:$PERF_PORT \
      --local > /tmp/auth_proxy_${INSTANCE}_$$.log 2>&1 &
  AUTH_PROXY_PID=$!
  echo "Auth proxy PID: $AUTH_PROXY_PID"
  sleep 2 # Give proxy time to start
fi

if [ "$RUN_BREAKGLASS" = true ]; then
  echo "Breakglass requested."
  read -p "Enter Bug Number (e.g. 123456): " BUG_NUM
  read -p "Enter Reason (e.g. debugging prod issue): " REASON_TEXT

  APPROVERS=""
  read -p "Do you want to assign a specific reviewer? [y/N] (default: auto-assign): " WANT_REVIEWER
  if [[ "$WANT_REVIEWER" =~ ^[Yy] ]]; then
      read -p "Enter Reviewer LDAP: " APPROVERS
  else
      echo "Using auto-assign."
  fi

  # Format: b/<bug> <reason>
  FULL_REASON="b/$BUG_NUM $REASON_TEXT"
  GRANTS_LOG="/tmp/grants_breakglass_$$.log"

  echo "Running grants command in background..."
  echo "Reason: $FULL_REASON"
  if [ -n "$APPROVERS" ]; then
      echo "Approvers: $APPROVERS"
  fi

  # Construct command arguments
  GRANTS_ARGS=(add --wait_for_twosync --reason="$FULL_REASON")
  if [ -n "$APPROVERS" ]; then
      GRANTS_ARGS+=(--preferred_approvers="$APPROVERS")
  fi
  GRANTS_ARGS+=("skia-infra-breakglass-policy:20h")

  # Run grants command in background and capture output
  grants "${GRANTS_ARGS[@]}" > "$GRANTS_LOG" 2>&1 &
  GRANTS_PID=$!

  echo "Waiting for approval URL..."

  # Poll the log file for the URL
  FOUND_URL=false
  while kill -0 "$GRANTS_PID" > /dev/null 2>&1; do
      if grep -q "https://" "$GRANTS_LOG"; then
          URL=$(grep -o "https://[^ ]*" "$GRANTS_LOG" | head -n 1)
          echo "--------------------------------------------------------"
          echo "Approval URL: $URL"
          echo "Please click the URL to approve the breakglass request."
          echo "--------------------------------------------------------"
          FOUND_URL=true
          break
      fi
      sleep 1
  done

  if [ "$FOUND_URL" = false ]; then
      echo "Grants command finished without providing a URL. Checking log..."
      cat "$GRANTS_LOG"
  fi
fi

# Initial build and run
attempt_build_and_start

echo "Watching for file changes in ./perf ..."
touch "$MARKER_FILE"
while true; do
  sleep 2
  CURRENT_TS=$(date +%s)
  # Watch for changes in Go files, BUILD files, and a few other common types
  CHANGED_FILES=$(find ./perf -type f \
    \( -name '*.go' -o -name 'BUILD.bazel' -o -name '*.html' -o -name '*.ts' \
    -o -name '*.scss' -o -name '*.json' -o -name '*.css' -o -name '*.png' -o -name '*.svg' \) \
    -newer "$MARKER_FILE")

  if [ -n "$CHANGED_FILES" ]; then
    echo "Changes detected in:"
    echo "$CHANGED_FILES"
    # Update marker file immediately to capture any changes that happen during rebuild
    touch "$MARKER_FILE"
    stop_server
    attempt_build_and_start
    continue
  fi

  # If server is NOT running, check if we should retry
  if [ "$SERVER_PID" -eq 0 ]; then
      TIME_SINCE_LAST=$(($CURRENT_TS - $LAST_BUILD_ATTEMPT))
      if [ "$TIME_SINCE_LAST" -ge "$RETRY_INTERVAL" ]; then
          echo "Retrying build/start after $RETRY_INTERVAL seconds..."
          attempt_build_and_start
          # Reset marker file on retry to avoid immediate re-trigger if nothing changed
          touch "$MARKER_FILE"
      fi
  else
      # Check if server died silently
      if ! kill -0 "$SERVER_PID" > /dev/null 2>&1; then
          echo "Perfserver process (PID: $SERVER_PID) disappeared. Scheduling retry."
          SERVER_PID=0
          # Will be retried on next loop iteration if enough time passed
      fi
  fi
done
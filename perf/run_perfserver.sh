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
android  $PUB  $SPAN_PUB  androidx        android2.json         $ASRC platform/frameworks/support
flutter  $PUB  $SPAN_PUB  flutter_flutter flutter-flutter.json  $GSRC flutter/flutter
widevine $CORP $SPAN_CORP widevine_cdm    widevine-cdm.json     $GSRC chromium/src
webrtc   $PUB  $SPAN_PUB  webrtc_pub      webrtc-public.json    $GSRC chromium/src
skia     $PUB  $SPAN_PUB  skia            skia-public.json      $SSRC skia
v8_pub   $PUB  $SPAN_PUB  v8              v8-public.json        $GSRC v8/v8
EOF

VALID_INSTANCES="${!PROJECTS[@]}"

usage() {
  echo "Usage: $0 --instance <instance_type> [--proxy]"
  echo "  Valid instance types are: ${VALID_INSTANCES// / | }"
  echo "  --proxy: Build and run the auth-proxy in the background."
  exit 1
}

INSTANCE=""
RUN_PROXY=false

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

LOCKFILE="/tmp/run_perfserver_${INSTANCE}.lock"
if [ -e "$LOCKFILE" ]; then
  LOCKED_PID=$(cat "$LOCKFILE")
  if kill -0 "$LOCKED_PID" > /dev/null 2>&1; then
    echo "Error: Lock file exists: $LOCKFILE"
    echo "Another instance of the script for '$INSTANCE' is running (PID: $LOCKED_PID)."
    rm -f "$TEMP_CONFIG"
    exit 1
  else
    echo "Found stale lock file for '$INSTANCE' (PID: $LOCKED_PID). Removing it."
    rm -f "$LOCKFILE"
  fi
fi
echo $$ > "$LOCKFILE"

# --- Port Availability Checks ---
# Aggressively clean up any Docker container using port 5432
docker ps -q --filter "publish=5432" | xargs -r docker rm -f >/dev/null 2>&1

for port in 8002 20001 5432; do
  if nc -z 127.0.0.1 $port > /dev/null 2>&1; then
    echo "Error: Port $port is already in use."
    echo "Please identify and stop the process using this port, or use a different port."
    rm -f "$LOCKFILE"
    rm -f "$TEMP_CONFIG"
    exit 1
  fi
done

echo "Using the following params: -p=$PROJECT -i=$SPANNER_INSTANCE -d=$DATABASE \
  -config=$TEMP_CONFIG -domain=$DOMAIN -repo=$REPO"

CONTAINER_NAME="perf-pgadapter-${INSTANCE}"
SERVER_PID=0
AUTH_PROXY_PID=0

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
  bazelisk build //perf/... --config=mayberemote -c dbg
}

start_server() {
  echo "Starting new perfserver in background..."
  LOG_FILE="/tmp/perfserver_${INSTANCE}_$(date +%Y%m%d_%H%M%S).log"
  echo "Log file: $LOG_FILE"

  _bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
      --dev_mode \
      --localToProd \
      --do_clustering=false \
      --port=:8002 \
      --prom_port=:20001 \
      --config_filename="$TEMP_CONFIG" \
      --display_group_by=false \
      --disable_metrics_update=true \
      --resources_dir=_bazel_bin/perf/pages/development/ \
      --connection_string="postgresql://root@127.0.0.1:5432/${DATABASE}?sslmode=disable" \
      --commit_range_url="https://${DOMAIN}/${REPO}/+log/{begin}..{end}" > "$LOG_FILE" 2>&1 &
  SERVER_PID=$!
  echo "Perfserver process started (PID: $SERVER_PID)"

  echo "Waiting for perfserver to be ready on port 8002..."
  MAX_TRIES=150
  COUNT=0
  while ! nc -z 127.0.0.1 8002 > /dev/null 2>&1; do
    if ! kill -0 $SERVER_PID > /dev/null 2>&1; then
      echo "Perfserver process (PID: $SERVER_PID) died unexpectedly."
      echo -e "\n--- Last 100 lines of perfserver log ---"
      tail -n 100 "$LOG_FILE"
      echo -e "\n--- Last 50 lines of pgadapter log ---"
      docker logs "$CONTAINER_NAME" 2>&1 | tail -n 50
      if docker logs "$CONTAINER_NAME" 2>&1 | \
         grep -q "PERMISSION_DENIED: Cloud Monitoring API has not been used in project"; then
         echo -e "\n*** DETECTED WRONG PROJECT CONFIGURATION ***"
         echo "Your credentials are likely configured with an incorrect quota project."
         echo "Please run the following command to fix it:"
         echo "  gcloud auth application-default set-quota-project $PROJECT"
         echo "**********************************************"
      fi
      echo "---------------------------------"
      exit 1
    fi
    sleep 2
    COUNT=$((COUNT + 1))
    if [ $COUNT -ge $MAX_TRIES ]; then
      echo "Perfserver failed to become ready after $(($MAX_TRIES * 2)) seconds."
      echo -e "\n--- Last 100 lines of perfserver log ---"
      tail -n 100 "$LOG_FILE"
      kill "$SERVER_PID" > /dev/null 2>&1
      exit 1
    fi
    echo -n "."
  done
  HOSTNAME=$(hostname -f)
  echo -e "\nPerfserver is up and running at http://${HOSTNAME}:8002"
  if [ "$RUN_PROXY" = true ]; then
    echo "Auth proxy is up and running at http://${HOSTNAME}:8003"
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
  fuser -k 8002/tcp >/dev/null 2>&1
  fuser -k 8003/tcp >/dev/null 2>&1
  fuser -k 20001/tcp >/dev/null 2>&1
  fuser -k 20003/tcp >/dev/null 2>&1

  if [ -n "$CONTAINER_NAME" ]; then
    echo "Stopping pgadapter container..."
    docker rm -f "$CONTAINER_NAME" > /dev/null 2>&1
  fi
  rm -f "$LOCKFILE"
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
docker rm -f "$CONTAINER_NAME" > /dev/null 2>&1

# Now let's run pgadapter connected to the supplied spanner database.
echo "Starting pgadapter..."
docker run -d --rm --name "$CONTAINER_NAME" -p 127.0.0.1:5432:5432 \
  -e GOOGLE_CLOUD_PROJECT="$PROJECT" \
  -e GOOGLE_CLOUD_QUOTA_PROJECT="$PROJECT" \
  -v "$ADC_FILE":/acct_credentials.json \
  gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
  -p "$PROJECT" -i "$SPANNER_INSTANCE" -d "$DATABASE" \
  -c /acct_credentials.json -x > /dev/null

echo "pgadapter container started: $CONTAINER_NAME"
echo "Waiting for pgadapter to be ready on port 5432..."
MAX_PG_TRIES=30
PG_COUNT=0
while ! nc -z 127.0.0.1 5432 > /dev/null 2>&1; do
  sleep 1
  PG_COUNT=$((PG_COUNT + 1))
  if [ $PG_COUNT -ge $MAX_PG_TRIES ]; then
    echo -e "\nError: pgadapter failed to become ready after $MAX_PG_TRIES seconds."
    if ! docker ps -q -f name="$CONTAINER_NAME" | grep -q .; then
       echo "pgadapter container died. Docker logs:"
       docker logs "$CONTAINER_NAME"
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
  echo "Clearing ports for auth-proxy..."
  fuser -k 20003/tcp || echo "Port 20003 was free or could not be cleared without sudo"
  fuser -k 8003/tcp || echo "Port 8003 was free or could not be cleared without sudo"

  echo "Building and starting auth proxy..."
  (cd "$PROJECT_ROOT/perf" && make run-auth-proxy-before-demo-instance) &
  AUTH_PROXY_PID=$!
  echo "Auth proxy PID: $AUTH_PROXY_PID"
  sleep 2 # Give proxy time to start
fi

# Initial build and run
if build_server; then
  start_server
else
  echo "Initial build failed. Exiting."
  cleanup
fi

echo "Watching for file changes in ./perf ..."
LAST_TS=$(date +%s)
while true; do
  # echo "Checking for file changes..."
  sleep 2
  CURRENT_TS=$(date +%s)
  # Watch for changes in Go files, BUILD files, and a few other common types
  CHANGED_FILES=$(find ./perf -type f \
    \( -name '*.go' -o -name 'BUILD.bazel' -o -name '*.html' -o -name '*.ts' \
    -o -name '*.scss' -o -name '*.json' \) \
    -newermt "@$LAST_TS")
  if [ -n "$CHANGED_FILES" ]; then
    echo "Changes detected in:"
    echo "$CHANGED_FILES"
    stop_server
    if build_server; then
      start_server
    else
      echo "Build failed. Not restarting server."
    fi
  fi
  LAST_TS=$CURRENT_TS
done

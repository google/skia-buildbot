#!/bin/bash
# run_manual_bisection_test.sh
# Complete developer orchestration script to spin up the local manual testing playground.

set -e

# Determine the workspace root directory and change to perf directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${WORKSPACE_ROOT}/perf"

TESTLOGS_DIR="${WORKSPACE_ROOT}/_bazel_testlogs"
mkdir -p "${TESTLOGS_DIR}"

# Enable monitor mode so background processes are run in their own process groups.
set -m

TEMPORAL_PID=""
BACKEND_PID=""
WORKER_PID=""
MOCKHOST_PID=""

cleanup() {
  echo ""
  echo "=== Shutting down background services ==="
  if [ -n "${WORKER_PID}" ]; then
    echo "Stopping worker (PGID: -${WORKER_PID})..."
    kill -TERM -"${WORKER_PID}" 2>/dev/null || true
  fi
  if [ -n "${BACKEND_PID}" ]; then
    echo "Stopping backendserver (PGID: -${BACKEND_PID})..."
    kill -TERM -"${BACKEND_PID}" 2>/dev/null || true
  fi
  if [ -n "${MOCKHOST_PID}" ]; then
    echo "Stopping mock issuetracker host (PGID: -${MOCKHOST_PID})..."
    kill -TERM -"${MOCKHOST_PID}" 2>/dev/null || true
  fi
  if [ -n "${TEMPORAL_PID}" ]; then
    echo "Stopping local Temporal server (PGID: -${TEMPORAL_PID})..."
    kill -TERM -"${TEMPORAL_PID}" 2>/dev/null || true
  fi
  echo "Clean shutdown complete."
}

# Trap exit signals to ensure background processes are cleaned up
trap cleanup SIGINT SIGTERM EXIT

echo "=== Step 0: Building Services Upfront ==="
echo "Building initdemo..."
bazelisk build --config=remote -c dbg //perf/go/initdemo:initdemo
echo "Building perfserver..."
bazelisk build --config=remote //perf/go/perfserver:perfserver
echo "Building mock issuetracker host..."
bazelisk build --config=remote //perf/go/issuetracker/mockhost:mock
echo "Building backendserver..."
bazelisk build --config=remote //perf/go/backend/backendserver:backendserver
echo "Building workflows worker..."
bazelisk build --config=remote //perf/go/workflows/worker:worker --@rules_go//go/config:tags=dev
echo "All services built successfully upfront."
echo ""

echo "=== Step 1: Checking local Temporal server ==="
if ! command -v temporal >/dev/null 2>&1; then
  echo "Error: 'temporal' CLI is not installed."
  echo "Please install it - see //pinpoint/readme.md for instructions."
  exit 1
fi

if ! nc -z localhost 7233; then
  echo "Temporal is not running. Starting local Temporal dev server in the background..."
  temporal server start-dev > "${TESTLOGS_DIR}/autobisection_e2e_test_temporal_server.log" 2>&1 &
  TEMPORAL_PID=$!
  # Wait for port 7233 to become active
  echo "Waiting for Temporal to start..."
  for i in {1..30}; do
    if nc -z localhost 7233; then
      break
    fi
    sleep 1
  done
  if ! nc -z localhost 7233; then
    echo "Temporal failed to start. See ${TESTLOGS_DIR}/autobisection_e2e_test_temporal_server.log"
    exit 1
  fi
  echo "Temporal server started successfully."
else
  echo "Temporal server is already running."
fi

echo "=== Step 2: Ensuring Temporal namespace 'perf-internal' exists ==="
if ! temporal operator namespace describe perf-internal >/dev/null 2>&1; then
  echo "Registering namespace 'perf-internal'..."
  temporal operator namespace create perf-internal
else
  echo "Namespace 'perf-internal' already exists."
fi

echo "=== Step 3: Preparing Database Schema & Mock Records ==="
./scripts/setup_manual_bisection_test_db.sh

echo "=== Step 3b: Launching Mock IssueTracker Host ==="
bazelisk run --config=remote //perf/go/issuetracker/mockhost:mock > "${TESTLOGS_DIR}/autobisection_e2e_test_mockhost.log" 2>&1 &
MOCKHOST_PID=$!

echo "Waiting for mock issuetracker to listen on port 8081..."
for i in {1..60}; do
  if nc -z localhost 8081; then
    break
  fi
  sleep 0.5
done
if ! nc -z localhost 8081; then
  echo "Error: mock issuetracker failed to start on port 8081."
  echo "See ${TESTLOGS_DIR}/autobisection_e2e_test_mockhost.log"
  exit 1
fi
echo "Mock issuetracker started (PID: ${MOCKHOST_PID})"

# Get absolute path to the config file to bypass Bazel run sandboxing
CONFIG_PATH="$(pwd)/configs/demo_spanner.json"

echo "=== Step 4: Launching backendserver with DevMode ==="
bazelisk run --config=remote //perf/go/backend/backendserver:backendserver -- run \
  --config_filename="${CONFIG_PATH}" \
  --port=:8005 \
  --dev_mode \
  --prom_port=:20002 > "${TESTLOGS_DIR}/autobisection_e2e_test_backendserver.log" 2>&1 &
BACKEND_PID=$!

# Wait for backendserver port to be active
echo "Waiting for backendserver to listen on port 8005..."
for i in {1..120}; do
  if nc -z localhost 8005; then
    break
  fi
  sleep 0.5
done
if ! nc -z localhost 8005; then
  echo "Error: backendserver failed to start on port 8005."
  echo "See ${TESTLOGS_DIR}/autobisection_e2e_test_backendserver.log"
  exit 1
fi
echo "backendserver started (PID: ${BACKEND_PID})"

echo "=== Step 5: Launching Workflows Worker with Mock Pinpoint ==="
bazelisk run --config=remote //perf/go/workflows/worker:worker --@rules_go//go/config:tags=dev -- \
  --hostPort=localhost:7233 \
  --namespace=perf-internal \
  --taskQueue=localhost.dev \
  --useMockPinpoint \
  --local > "${TESTLOGS_DIR}/autobisection_e2e_test_worker.log" 2>&1 &
WORKER_PID=$!
echo "Workflows worker started (PID: ${WORKER_PID})"

echo "========================================================"
echo " MANUAL WORKFLOW TESTING SANDBOX IS READY!"
echo "========================================================"
echo "You can trigger the workflow by executing the command below in another terminal:"
echo ""
echo "temporal workflow start \\"
echo "  --namespace perf-internal \\"
echo "  --task-queue localhost.dev \\"
echo "  --type perf.maybe_trigger_bisection \\"
echo "  --workflow-id manual-test-bisection \\"
echo "  --input '{"
echo "    \"AnomalyGroupServiceUrl\": \"localhost:8005\","
echo "    \"AutobisectionServiceUrl\": \"localhost:8005\","
echo "    \"CulpritServiceUrl\": \"localhost:8005\","
echo "    \"AnomalyGroupId\": \"a9d70df7-ff99-4720-9988-cb9470987114\","
echo "    \"GroupingTaskQueue\": \"localhost.dev\","
echo "    \"PinpointTaskQueue\": \"localhost.dev\","
echo "    \"WaitTimeForAnomalyClusteringWindow\": 5000000000,"
echo "    \"PinpointPollInterval\": 2000000000"
echo "  }'"
echo ""
echo "Monitor executions in Temporal Web UI:"
echo "  - Local: http://localhost:8233/namespaces/perf-internal/workflows"
echo "--------------------------------------------------------"
echo "Press Ctrl+C to terminate services and exit."
echo "--------------------------------------------------------"

# Keep script running to maintain background processes
while true; do
  sleep 1
done

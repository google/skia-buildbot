#!/bin/bash
# Runs a perf instance against the given spanner database.

# Example Command:
# ./run_with_spanner.sh p=skia-infra-corp i=tfgen-spanid-20241205020733610 \
#   d=v8_int_autopush config=./configs/spanner/v8-internal-autopush.json
#
# Required Parameters:
#   p      - GCP Project ID (e.g., skia-infra-corp)
#   i      - Spanner Instance ID (e.g., tfgen-spanid-20241205020733610)
#   d      - Spanner Database Name (e.g., v8_int_autopush)
#   config - Path to perfserver configuration JSON (e.g., ./configs/spanner/v8-internal-autopush.json)
#
# Optional Parameters & Flags:
#   domain              - Gitiles domain (default: chromium.googlesource.com)
#   repo                - Gitiles repository path (default: chromium/src)
#   --no-launch-browser - Require copy-pasting an auth code (works on remote machines/Cloudtop) [default]
#   --launch-browser    - Use local browser redirect flow (only works when running on a local machine)

# Ensure background processes (like auth-proxy) are cleaned up when the script exits
trap 'echo -e "\nStopping background processes..."; kill $(jobs -p) 2>/dev/null' EXIT

# Parse all arguments passed in
LAUNCH_BROWSER_FLAG="--no-launch-browser"
for arg in "$@"; do
  case "$arg" in
    --no-launch-browser)
      LAUNCH_BROWSER_FLAG="--no-launch-browser"
      ;;
    --launch-browser)
      LAUNCH_BROWSER_FLAG="--launch-browser"
      ;;
    *=*)
      argKey=$(echo "$arg" | cut -f1 -d=)
      keyLen=${#argKey}
      val="${arg:$keyLen+1}"
      export "$argKey"="$val"
      ;;
  esac
done

# --- Credential check ---
echo "Checking Google Cloud credentials for apps (ADC)..."
if ! gcloud auth application-default print-access-token &>/dev/null; then
  echo "ADC credentials missing or expired. Launching login flow..."
  gcloud auth application-default login "$LAUNCH_BROWSER_FLAG"

  # Double check if the login succeeded
  if ! gcloud auth application-default print-access-token &>/dev/null; then
    echo "ERROR: Login failed or was cancelled. Exiting."
    exit 1
  fi
else
  echo "ADC credentials are valid."
fi
# --------------------------------------

# First delete any existing docker containers to start clean.
sudo docker ps -q | xargs -r sudo docker rm -vf

# Check if domain or repo are set via params, if not set them to default values.
if [[ -z "${domain}" ]]; then
  domain="chromium.googlesource.com"
fi

if [[ -z "${repo}" ]]; then
  repo="chromium/src"
fi

echo "Using the following params: -p=$p -i=$i -d=$d -config=$config -domain=$domain -repo=$repo"

# Now let's run pgadapter connected to the supplied spanner database.
sudo docker run -d -p 127.0.0.1:5432:5432 \
  -e JAVA_TOOL_OPTIONS="-Xms2g -Xmx2g -XX:+UseG1GC -XX:+ExitOnOutOfMemoryError" \
  -v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json \
  gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
  -p $p -i $i -d $d -c /acct_credentials.json -x

# Build the perfserver, frontend pages, and auth-proxy.
bazelisk build --config=mayberemote -c dbg //perf/go/perfserver //perf/pages/... //kube/cmd/auth-proxy || {
  echo "ERROR: Build failed, exiting (not trying to run outdated version)."
  exit 1
}

# Start the auth-proxy in the background (&)
echo "Starting auth-proxy in the background..."
../_bazel_bin/kube/cmd/auth-proxy/auth-proxy_/auth-proxy \
  --prom-port=:20003 \
  --role=editor=google.com \
  --authtype=mocked \
  --mock_user="${USER}@google.com" \
  --port=:8003 \
  --target_port=http://127.0.0.1:8002 \
  --local &

echo "Auth-proxy started on port :8003 - access this instead of :8002 to access anomalies."

# Now that pgadapter and auth-proxy are up, let's run the local frontend
echo "Starting perfserver..."
../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
  --dev_mode \
  --localToProd \
  --do_clustering=false \
  --port=:8002 \
  --prom_port=:20001 \
  --config_filename=$config \
  --display_group_by=false \
  --disable_metrics_update=true \
  --resources_dir=../_bazel_bin/perf/pages/development/ \
  --connection_string=postgresql://root@127.0.0.1:5432/${d}?sslmode=disable \
  --commit_range_url=https://${domain}/${repo}/+log/{begin}..{end}

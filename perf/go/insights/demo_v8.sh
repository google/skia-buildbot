#!/bin/bash
# Runs the perfinsights demo on this Cloudtop against the production V8 Perf database, read-only.
#
# Usage, from any directory:
#   perf/go/insights/demo_v8.sh           Serve on http://127.0.0.1:8090, for this machine only.
#   perf/go/insights/demo_v8.sh --share   Also serve other Googlers through UberProxy, at
#                                         http://<this cloudtop>:8090.
#   perf/go/insights/demo_v8.sh --stop    Stop the PGAdapter container.
#
# Environment overrides: DATABASE (v8_int or v8_int_autopush), PORT (8090), PG_PORT (5433).
#
# Prerequisites: Docker, read access to the database (Perf developers have it, see
# perf/Spanner.md) and Application Default Credentials: gcloud auth application-default login
#
# Safety: the database is only reachable through a PGAdapter bound to 127.0.0.1, every connection
# is read-only (default_transaction_read_only), and perfinsights only uses read methods. FIX-01
# (Gate G1) hasn't landed, so perfinsights validates every id as a UUID before it reaches the
# store's GetByIDs, which still interpolates ids into its SQL.
set -euo pipefail

PROJECT="skia-infra-corp"
INSTANCE="tfgen-spanid-20241205020733610"
DATABASE="${DATABASE:-v8_int}"
PORT="${PORT:-8090}"
PG_PORT="${PG_PORT:-5433}"
PGADAPTER_IMAGE="gcr.io/cloud-spanner-pg-adapter/pgadapter:latest"
CONTAINER="perfinsights-pgadapter-${DATABASE//_/-}"
ADC="${HOME}/.config/gcloud/application_default_credentials.json"
PERF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BAZEL="${BAZEL:-bazelisk}"

case "${DATABASE}" in
  v8_int) CONFIG="${PERF_DIR}/configs/spanner/v8-internal.json" ;;
  v8_int_autopush) CONFIG="${PERF_DIR}/configs/spanner/v8-internal-autopush.json" ;;
  *)
    echo "DATABASE must be v8_int or v8_int_autopush." >&2
    exit 1
    ;;
esac

stop_pgadapter() {
  if docker rm -f "${CONTAINER}" >/dev/null 2>&1; then
    echo "Stopped ${CONTAINER}."
  else
    echo "${CONTAINER} wasn't running."
  fi
}

start_pgadapter() {
  if docker ps --format '{{.Names}}' | grep -qx "${CONTAINER}"; then
    echo "PGAdapter is already running (${CONTAINER})."
    return
  fi
  # Only this script's own container is ever removed.
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
  echo "Starting PGAdapter for ${PROJECT}/${INSTANCE}/${DATABASE} on 127.0.0.1:${PG_PORT}..."
  docker run -d --name "${CONTAINER}" -p "127.0.0.1:${PG_PORT}:5432" \
    -v "${ADC}:/acct_credentials.json:ro" \
    "${PGADAPTER_IMAGE}" \
    -p "${PROJECT}" -i "${INSTANCE}" -d "${DATABASE}" -c /acct_credentials.json -x >/dev/null
  for _ in $(seq 1 60); do
    if (echo >"/dev/tcp/127.0.0.1/${PG_PORT}") 2>/dev/null; then
      return
    fi
    sleep 1
  done
  echo "PGAdapter didn't start. See: docker logs ${CONTAINER}" >&2
  exit 1
}

SHARE=false
for arg in "$@"; do
  case "${arg}" in
    --share) SHARE=true ;;
    --stop)
      stop_pgadapter
      exit 0
      ;;
    -h | --help)
      sed -n '2,18p' "$0"
      exit 0
      ;;
    *)
      echo "Unknown argument: ${arg}. See --help." >&2
      exit 1
      ;;
  esac
done

if [[ ! -f "${ADC}" ]]; then
  echo "No Application Default Credentials. Run: gcloud auth application-default login" >&2
  exit 1
fi
if ! command -v "${BAZEL}" >/dev/null; then
  BAZEL="${HOME}/go/bin/bazelisk"
fi

start_pgadapter

FLAGS=(
  --config_filename="${CONFIG}"
  --connection_string="postgresql://root@127.0.0.1:${PG_PORT}/${DATABASE}?sslmode=disable"
  --allow_non_demo_db
  --visibility=internal
)
if [[ "${SHARE}" == true ]]; then
  # Every request arrives from UberProxy's addresses, so the per-caller limit is shared.
  FLAGS+=(--listen=":${PORT}" --uberproxy_pen --rate_limit_qps=20 --rate_limit_burst=60)
  echo
  echo "Share this URL with the team. It opens in Chrome on corp laptops, through UberProxy:"
  echo "  http://$(hostname -f):${PORT}"
  echo "Anyone with it can read ${DATABASE}. Stop the server (Ctrl+C) after the demo."
  echo
else
  FLAGS+=(--listen="127.0.0.1:${PORT}")
fi

cd "${PERF_DIR}/.."
exec "${BAZEL}" run --config=mayberemote //perf/go/insights/insightsserver -- "${FLAGS[@]}"

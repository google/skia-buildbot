#!/bin/sh
#
# Modified from:
# https://github.com/temporalio/docker-builds/blob/9f2a59c6d2dd979b2afb8a83b221769b33557cd8/docker/entrypoint.sh#L1

set -eu -o pipefail

source /etc/temporal/env-default.sh

: "${BIND_ON_IP:=$(getent hosts "$(hostname)" | awk '{print $1;}')}"
export BIND_ON_IP

# If binding to wildcard address (0.0.0.0 or ::0), set broadcast address if not already set
if [ "${BIND_ON_IP}" = "0.0.0.0" ] || [ "${BIND_ON_IP}" = "::0" ]; then
    : "${TEMPORAL_BROADCAST_ADDRESS:=$(getent hosts "$(hostname)" | awk '{print $1;}')}"
    export TEMPORAL_BROADCAST_ADDRESS
fi

# check TEMPORAL_ADDRESS is not empty
if [[ -z "${TEMPORAL_ADDRESS:-}" ]]; then
    echo "TEMPORAL_ADDRESS is not set, setting it to ${BIND_ON_IP}:7233"

    if echo "${BIND_ON_IP}" | grep -Eq ":"; then
        # ipv6
        export TEMPORAL_ADDRESS="[${BIND_ON_IP}]:7233"
    else
        # ipv4
        export TEMPORAL_ADDRESS="${BIND_ON_IP}:7233"
    fi
fi

source /etc/temporal/utils.sh

for arg; do
  case ${arg} in
    init-db)
      validate_db_env
      wait_for_postgres
      setup_postgres_schema
      ;;
    setup-server)
      setup_server
      ;;
    start-server)
      start_server
      ;;
  esac
done

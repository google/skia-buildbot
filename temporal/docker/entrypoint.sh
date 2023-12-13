#!/bin/sh
#
# Modified from:
# https://github.com/temporalio/docker-builds/blob/9f2a59c6d2dd979b2afb8a83b221769b33557cd8/docker/entrypoint.sh#L1

set -eu -o pipefail

: "${BIND_ON_IP:=$(getent hosts "$(hostname)" | awk '{print $1;}')}"
export BIND_ON_IP

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

/etc/temporal/dockerize -template /etc/config_template.yaml:/etc/temporal/config/docker.yaml

exec /etc/temporal/temporal-server --env docker start

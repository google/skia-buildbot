#!/bin/bash
set -e
set -o pipefail

# Retrieve the most recent version of the secret from GCP Secret Manager.

if [ $# -ne 2 ]; then
    echo "$0 <secret name> <destination>"
    exit 1
fi

SECRET_NAME="$1"; shift
DEST="$1"; shift

REL=$(dirname "$0")
source ${REL}/config.sh

${REL}/get-gcp-secret.sh ${SECRET_NAME} > "${DEST}"

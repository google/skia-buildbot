#!/bin/bash
set -e
set -o pipefail

# Retrieve the most recent version of the service account key from GCP Secret Manager.

if [ $# -ne 3 ]; then
    echo "$0 <project id> <service-account-name> <destination file>"
    exit 1
fi

PROJECT="$1"; shift
SA_NAME="$1"; shift
DEST="$1"; shift

REL=$(dirname "$0")
${REL}/get-service-account-key.sh ${PROJECT} ${SA_NAME} > ${DEST}

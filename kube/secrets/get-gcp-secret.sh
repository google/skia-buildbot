#!/bin/bash
set -e
set -o pipefail

# Retrieve the most recent version of the secret from GCP Secret Manager.

if [ $# -ne 1 ]; then
    echo "$0 <secret name>"
    exit 1
fi

SECRET_NAME="$1"; shift

gcloud --project=skia-infra-public secrets versions access latest --secret=${SECRET_NAME}

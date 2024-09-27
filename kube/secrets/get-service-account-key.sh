#!/bin/bash
set -e
set -o pipefail

# Retrieve the most recent version of the service account key from GCP Secret Manager.

if [ $# -ne 2 ]; then
    echo "$0 <project id> <service-account-name>"
    exit 1
fi

PROJECT="$1"; shift
SA_NAME="$1"; shift

REL=$(dirname "$0")
source ${REL}/config.sh

SECRET_NAME="$(service_account_secret_name ${PROJECT} ${SA_NAME})"

${REL}/get-gcp-secret.sh ${SECRET_NAME}

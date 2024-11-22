#!/bin/bash
set -e
set -o pipefail

# Generate a new key for a service account and upload it to GCP Secret Manager.

if [ $# -ne 2 ]; then
    echo "$0 <project id> <service-account-name>"
    exit 1
fi

PROJECT="$1"; shift
SA_NAME="$1"; shift

REL=$(dirname "$0")
source ${REL}/config.sh

SECRET_NAME="$(service_account_secret_name ${PROJECT} ${SA_NAME})"

# Convert PROJECT to PROJECT_SUBDOMAIN, i.e. convert "google.com:skia-corp" to
# "skia-corp.google.com", but leave "skia-public" alone.
PROJECT_SUBDOMAIN=$(echo ${PROJECT} | sed 's#^\(.*\):\(.*\)$#\2.\1#g')
EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

# Create the secret if it doesn't already exist.
gcloud --project=skia-infra-public secrets create ${SECRET_NAME} >> /dev/null 2>&1 || true

# Create the key and add the new secret version.
gcloud --project=${PROJECT} beta iam service-accounts keys create /dev/stdout --iam-account="${EMAIL}" \
| gcloud --project=skia-infra-public secrets versions add ${SECRET_NAME} --data-file=-

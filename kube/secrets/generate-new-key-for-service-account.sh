#!/bin/bash
set -e
set -o pipefail

# Generate a new key for a service account and upload it to berglas.
#
# Note that the service account name and secret name are the same, which is the
# same assumption that add-service-account-from-stdin.sh makes.
if [ $# -ne 3 ]; then
    echo "$0 <project id> <cluster-name> <service-account-name>"
    exit 1
fi

PROJECT="$1"; shift
CLUSTER="$1"; shift
SECRET_NAME="$1"; shift

# Convert PROJECT to PROJECT_SUBDOMAIN, i.e. convert "google.com:skia-corp" to
# "skia-corp.google.com", but leave "skia-public" alone.
PROJECT_SUBDOMAIN=$(echo ${PROJECT} | sed 's#^\(.*\):\(.*\)$#\2.\1#g')

EMAIL="${SECRET_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

REL=$(dirname "$0")

source ${REL}/config.sh

gcloud beta iam service-accounts keys create /dev/stdout --iam-account="${EMAIL}" \
| ${REL}/add-service-account-from-stdin.sh ${CLUSTER} ${SECRET_NAME} 1>&2
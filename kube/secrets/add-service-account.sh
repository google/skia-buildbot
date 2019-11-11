#!/bin/bash

# Add a secret to berglas from stdin.
#
# The stdin stream should be a base64 encoded kubernetes secret file formatted
# as YAML.

set -x

if [ $# -le 3 ]; then
    echo "$0 <project id> <cluster-name> <service-account-name> <description> [<roles>*]"
    exit 1
fi

PROJECT="$1"; shift
CLUSTER="$1"; shift
SECRET_NAME="$1"; shift
DESCRIPTION="$1"; shift

# Create the service account.
gcloud iam service-accounts create "${SECRET_NAME}" --project=${PROJECT} --display-name="${DESCRIPTION}"

PROJECT_SUBDOMAIN=$(echo ${PROJECT} | sed 's#^\(.*\):\(.*\)$#\2.\1#g')
EMAIL="${SECRET_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

# Add the roles.
for role in $@; do
    gcloud projects add-iam-policy-binding "${PROJECT}" \
    --member "serviceAccount:$EMAIL" \
    --role ${role}
done

REL=$(dirname "$0")
source ${REL}/config.sh

gcloud beta iam service-accounts keys create /dev/stdout --iam-account="${EMAIL}" | ${REL}/add-service-account-from-stdin.sh ${CLUSTER} ${SECRET_NAME}
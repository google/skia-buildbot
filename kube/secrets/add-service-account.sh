#!/bin/bash
set -e
set -o pipefail

# Add a service account to berglas.
#
# The stdin stream should be a base64 encoded kubernetes secret file formatted
# as YAML.
#
# The script echos the full service account email to stdout.

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

# Convert PROJECT to PROJET_SUBDOMAIN, i.e. convert "google.com:skia-corp" to
# "skia-corp.google.com", but leave "skia-public" alone.
PROJECT_SUBDOMAIN=$(echo ${PROJECT} | sed 's#^\(.*\):\(.*\)$#\2.\1#g')

EMAIL="${SECRET_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

# Add the roles to service account.
for role in $@; do
    gcloud projects add-iam-policy-binding "${PROJECT}" \
    --member "serviceAccount:$EMAIL" \
    --role ${role} \
    --user-output-enabled=false
done

REL=$(dirname "$0")
source ${REL}/config.sh

gcloud beta iam service-accounts keys create /dev/stdout --iam-account="${EMAIL}" \
| ${REL}/add-service-account-from-stdin.sh ${CLUSTER} ${SECRET_NAME} 1>&2

echo ${EMAIL}
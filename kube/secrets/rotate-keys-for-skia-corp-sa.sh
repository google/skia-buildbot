#!/bin/bash
source ../kube/corp-config.sh

set -o pipefail

# Rotate the key for a service account. Create a new key, upload it to berglas,
# delete all older keys, and then restart all services that depend on that key.
#
# Note that the service account name and secret name are the same, which is the
# same assumption that add-service-account-from-stdin.sh makes.
if [ $# -ne 3 ]; then
    echo "$0 <project id> <service-account-name> <restart>"
    echo "For example, to rotate the key for a service account in skia-public used by the emailservice: "
    echo ""
    echo "    $0 skia-public skia-emailservice deployment/emailservice"
    exit 1
fi

which jq > /dev/null
if [ $? -ne 0 ]; then
  echo "jq needs to be installed"
  exit 1
fi

which berglas > /dev/null
if [ $? -ne 0 ]; then
  echo "berglas needs to be installed:  go install github.com/GoogleCloudPlatform/berglas@latest"
  exit 1
fi

set -e
set -x

# This is fixed to skia-corp since all other clusters should be using workload
# identity.
CLUSTER="skia-corp"

PROJECT="$1"; shift
SECRET_NAME="$1"; shift
RESTART="$1"; shift

# Convert PROJECT to PROJECT_SUBDOMAIN, i.e. convert "google.com:skia-corp" to
# "skia-corp.google.com", but leave "skia-public" alone.
PROJECT_SUBDOMAIN=$(echo ${PROJECT} | sed 's#^\(.*\):\(.*\)$#\2.\1#g')

EMAIL="${SECRET_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

REL=$(dirname "$0")

source ${REL}/config.sh


# Create new key.
${REL}/generate-new-key-for-service-account.sh ${PROJECT} ${CLUSTER} ${SECRET_NAME}

# Push new key to cluster.
${REL}/apply-secret-to-cluster.sh ${CLUSTER} ${SECRET_NAME}

# Remove all old keys

# List all USER_MANAGED keys, remove the last one in the list (which is the most recent since
# we sort by validBeforeTime), and then remove each of those keys.
gcloud iam service-accounts keys list --project=${PROJECT} --iam-account="${EMAIL}" --format=json --filter=keyType=USER_MANAGED --sort-by=validBeforeTime | jq '.[:-1]' | jq .[].name | xargs -L 1 gcloud iam service-accounts keys delete --project=${PROJECT} --iam-account=${EMAIL}

# Restart pods to pick up new keys."
${REL}/../attach.sh ${CLUSTER} kubectl rollout restart ${RESTART}

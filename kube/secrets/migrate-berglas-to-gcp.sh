#!/bin/bash
set -e
set -o pipefail

# Retrieve a secret from berglas, decode it, extract part of the YAML that is
# then further base64 decoded and import it to GCP secrets.
#
# The yaml path uses `yq` path notation to pull out elements. For example, to
# pull out the service account key from a kubernetes secret you would pass
# '.data."key.json"' as the YAML_PATH.

if [ $# -ne 4 ]; then
    echo "$0 <cluster name> <src secret name> <yaml path> <dest secret name>"
    exit 1
fi

CLUSTER=$1
SRC_SECRET_NAME=$2
YAML_PATH=$3
DEST_SECRET_NAME=$4

REL=$(dirname "$0")
source ${REL}/config.sh

# Create the secret but don't fail if it already exists.
gcloud --project=skia-infra-public secrets create $DEST_SECRET_NAME || true

berglas access ${BUCKET_ID}/${CLUSTER}/${SRC_SECRET_NAME} \
| base64 --decode \
| yq e ${YAML_PATH} - \
| base64 -d \
| gcloud --project=skia-infra-public secrets versions add $DEST_SECRET_NAME --data-file=-

#!/bin/bash
set -e
set -o pipefail

# Retrieve a secret from berglas, decode it, extract part of the YAML that is
# then further base64 decoded and written to an output file.
#
# The yaml path uses `yq` path notation to pull out elements. For example, to
# pull out the service account key from a kubernetes secret you would pass
# '.data."key.json"' as the YAML_PATH.

if [ $# -ne 4 ]; then
    echo "$0 <cluster-name> <secret-name> <yaml path> <output file>"
    exit 1
fi

CLUSTER=$1
SECRET_NAME=$2
YAML_PATH=$3
FILENAME=$4

REL=$(dirname "$0")
source ${REL}/config.sh

berglas access ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} \
| base64 --decode \
| yq e ${YAML_PATH} - \
| base64 -d > ${FILENAME}

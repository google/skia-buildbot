#!/bin/bash
set -e
set -o pipefail

# Retrieve a secret from berglas.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET_NAME=$2

REL=$(dirname "$0")
source ${REL}/config.sh

berglas access ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} \
| base64 --decode

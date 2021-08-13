#!/bin/bash
set -e

# Delete a secret from berglas.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET_NAME=$2

REL=$(dirname "$0")
source ${REL}/config.sh

berglas delete ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME}

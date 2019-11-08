#!/bin/bash

# Add a secret to berglas.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET_NAME=$2

REL=$(dirname "$0")
source ${REL}/config.sh

berglas create ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} - --key ${KEY}

berglas grant ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} --member group:skia-root@google.com
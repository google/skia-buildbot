#!/bin/bash

# Add a secret to berglas.

if [ $# ne 3 ]; then
    echo "$0 <path-to-file-to-store> <cluster-name> <secret-name>"
fi

FILE=$1
CLUSTER=$2
SECRET_NAME=$3

source ./config.sh

berglas create ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} ${FILE} --key ${KEY}

berglas grant ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} --member group:skia-root@google.com
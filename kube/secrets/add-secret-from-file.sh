#!/bin/bash

# Add a secret to berglas, replaces secret if it already exists.

if [ $# -ne 3 ]; then
    echo "$0 <path-to-file> <cluster-name> <secret-name>"
    exit 1
fi

FILE=$1
CLUSTER=$2
SECRET_NAME=$3

REL=$(dirname "$0")
source ${REL}/config.sh

berglas update ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} @${FILE} --create-if-missing --key ${KEY}

berglas grant ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} --member group:skia-root@google.com
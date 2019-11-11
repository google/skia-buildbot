#!/bin/bash

# Add a secret to berglas from stdin.
#
# The stdin stream should be a base64 encoded kubernetes secret file formatted
# as YAML.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET_NAME=$2

REL=$(dirname "$0")
source ${REL}/config.sh

berglas update ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} - --create-if-missing --key ${KEY}

berglas grant ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} ${ACCESS_CONTROL}
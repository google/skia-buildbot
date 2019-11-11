#!/bin/bash

# Add a secret to berglas from all the files in the given directory, replaces secret if it already exists.

if [ $# -ne 3 ]; then
    echo "$0 <directory> <cluster-name> <secret-name>"
    exit 1
fi

DIRECTORY=$1
CLUSTER=$2
SECRET_NAME=$3

REL=$(dirname "$0")
source ${REL}/config.sh

# bergals only understands a single line, so we base64 encode the whole file,
# and then use awk to add a single newline to the end of the base64, which
# berglas also needs.
kubectl create secret generic ${SECRET_NAME} --from-file=${DIRECTORY}  --dry-run -o yaml | base64 --wrap=0 | awk '{ print $0 }' | berglas update ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} - --create-if-missing --key ${KEY}

berglas grant ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} ${ACCESS_CONTROL}
#!/bin/bash
set -e
set -o pipefail

# Add a secret to berglas from all the files in the given directory, replaces
# secret if it already exists.

if [ $# -ne 3 ]; then
    echo "$0 <directory> <cluster-name> <secret-name>"
    exit 1
fi

DIRECTORY=$1
CLUSTER=$2
SECRET_NAME=$3

REL=$(dirname "$0")
source ${REL}/config.sh

kubectl create secret generic ${SECRET_NAME} --from-file=${DIRECTORY}  --dry-run -o yaml \
| ${REL}/add-secret-from-stdin.sh ${CLUSTER} ${SECRET_NAME}
#!/bin/bash
set -e
set -o pipefail

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

kubectl create secret generic "${SECRET_NAME}" --from-file=key.json=/dev/stdin --dry-run=client -o yaml \
| ${REL}/add-secret-from-stdin.sh ${CLUSTER} ${SECRET_NAME}
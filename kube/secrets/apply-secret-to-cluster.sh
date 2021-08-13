#!/bin/bash
set -e
set -o pipefail

# Push secrets from one cluster to kuberenetes.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET=$2

REL=$(dirname "$0")
source ${REL}/config.sh

confirm_cluster "$CLUSTER"

${REL}/get-secret.sh ${CLUSTER} ${SECRET} | kubectl apply -f -

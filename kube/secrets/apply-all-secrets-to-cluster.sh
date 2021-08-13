#!/bin/bash
set -e
set -o pipefail

# Push secrets from one cluster to kuberenetes.

if [ $# -ne 1 ]; then
    echo "$0 <cluster-source-name>"
    exit 1
fi

CLUSTER=$1

REL=$(dirname "$0")
source ${REL}/config.sh

confirm_cluster "$CLUSTER"

LIST=$(${REL}/list-secrets-by-cluster.sh ${CLUSTER})
echo ${LIST}

for NAME in ${LIST[@]}
do
  ${REL}/get-secret.sh ${CLUSTER} ${NAME} | kubectl apply -f -
done

#!/bin/bash

# Push secrets from one cluster to kuberenetes.

if [ $# -ne 1 ]; then
    echo "$0 <cluster-source-name>"
    exit 1
fi

CLUSTER=$1

# TBD - In the future this script will know how to switch among all the clusters.
K8S_CLUSTER=$(kubectl config current-context)
if [ "$K8S_CLUSTER" != "$CLUSTER" ]
then
  echo "Wrong cluster, must be run in $CLUSTER."
  exit 1
fi

REL=$(dirname "$0")
source ${REL}/config.sh

LIST=$(${REL}/list-secrets-by-cluster.sh ${CLUSTER})
echo ${LIST}

for NAME in ${LIST[@]}
do
  # ${REL}/get-secret.sh ${CLUSTER} ${NAME} | kubectl create secret generic "${NAME}" --from-file=key.json=/dev/stdin
  ${REL}/get-secret.sh ${CLUSTER} ${NAME} | kubectl create secret generic "${NAME}" --from-file=key.json=/dev/stdin
done




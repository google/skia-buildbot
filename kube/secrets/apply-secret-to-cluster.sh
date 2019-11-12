#!/bin/bash

# Push secrets from one cluster to kuberenetes.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET=$2

# TBD - In the future this script will know how to switch among all the clusters.
K8S_CLUSTER=$(kubectl config current-context)
if [ "$K8S_CLUSTER" != "$CLUSTER" ]
then
  echo "Wrong cluster, must be run in $CLUSTER."
  exit 1
fi

REL=$(dirname "$0")
source ${REL}/config.sh

${REL}/get-secret.sh ${CLUSTER} ${SECRET} | kubectl apply -f -

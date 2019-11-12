#!/bin/bash

# Copy secrets from one cluster into berglas.

if [ $# -ne 1 ]; then
    echo "$0 <cluster-source-name>"
    exit 1
fi

CLUSTER=$1

K8S_CLUSTER=$(kubectl config current-context)
if [ "$K8S_CLUSTER" != "$CLUSTER" ]
then
  echo "Wrong cluster, must be run in $CLUSTER."
  exit 1
fi

REL=$(dirname "$0")
source ${REL}/config.sh

LIST=$( kubectl get secrets --field-selector type==Opaque -o go-template='{{range .items}}{{.metadata.name}} {{end}}')

for NAME in ${LIST[@]}
do
  kubectl get secret ${NAME} -o yaml | ${REL}/add-secret-from-stdin.sh ${CLUSTER} ${NAME}
done
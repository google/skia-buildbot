#!/bin/bash
set -e
set -o pipefail

# Copy secrets from one cluster into berglas.

if [ $# -ne 1 ]; then
    echo "$0 <cluster-source-name>"
    exit 1
fi

CLUSTER=$1

REL=$(dirname "$0")
source ${REL}/config.sh

confirm_cluster ${CLUSTER}

LIST=$(kubectl get secrets --field-selector type==Opaque \
  -o go-template='{{range .items}}{{.metadata.name}} {{end}}')

for NAME in ${LIST[@]}
do
  kubectl get secret ${NAME} -o yaml \
  | ${REL}/add-secret-from-stdin.sh ${CLUSTER} ${NAME}
done
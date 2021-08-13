#!/bin/bash
set -e
set -o pipefail

# Copy secrets from one cluster to another in berglas.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-source-name> <cluster-dest-name>"
    exit 1
fi

SRC=$1
DST=$2

REL=$(dirname "$0")
source ${REL}/config.sh

LIST=$(${REL}/list-secrets-by-cluster.sh ${SRC})
echo ${LIST}

for NAME in ${LIST[@]}
do
  ${REL}/get-secret.sh ${SRC} ${NAME} \
  | ${REL}/add-secret-from-stdin.sh ${DST} ${NAME}
done
#!/bin/bash

# Copy secrets from one cluster to another in berglas.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-source-name> <cluster-dest-name>"
    exit 1
fi

SRC=$1
DST=$2

source ./config.sh

LIST=$(./list-secrets-by-cluster.sh ${SRC})
echo ${LIST}

for NAME in ${LIST[@]}
do
  ./get-secret.sh ${SRC} ${NAME} | ./add-secret-from-stdin.sh ${DST} ${NAME}
done
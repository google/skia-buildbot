#!/bin/bash
set -e
set -o pipefail

# Copy secret from one cluster to another in berglas.

if [ $# -ne 3 ]; then
    echo "$0 <cluster-source-name> <cluster-dest-name> <secret-name>"
    exit 1
fi

SRC=$1
DST=$2
SECRET=$3

REL=$(dirname "$0")
source ${REL}/config.sh

${REL}/get-secret.sh ${SRC} ${SECRET} \
  | ${REL}/add-secret-from-stdin.sh ${DST} ${SECRET}

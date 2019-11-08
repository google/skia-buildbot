#!/bin/bash

# List all secrets available.

source ./config.sh

if [ $# -ne 1 ]; then
    echo "$0 <cluster-name>"
    exit 1
fi

CLUSTER=$1

berglas list ${BUCKET_ID} --prefix=${CLUSTER}/ | tail -n +2 - |  awk '{print $1}' | sed s#${CLUSTER}/##g

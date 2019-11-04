#!/bin/bash

set -e -x

DIR=`mktemp -d`
printf ${DIR}
ssh chrome-bot@100.115.95.135 "sudo kubectl config view --raw" > ${DIR}/config
export KUBECONFIG=${DIR}/config
ssh -N -L 6443:localhost:6443 chrome-bot@100.115.95.135 &
PID=$!
/bin/bash
kill ${PID}
#!/bin/bash

# Creates a shell where kubectl is hooked up to the cluster.

set -e -x

CLUSTER=skolo-rpi2-01

DIR=${HOME}/.config/skia-infra/skolo/${skolo-rack4}

# Create dir to store config.
mkdir -p ${DIR}

# Grab config from the kubernetes cluster and store in the temp dir.
ssh chrome-bot@100.115.95.135 "sudo kubectl config view --raw" > ${DIR}/config

# Make kubectl use that config.
export KUBECONFIG=${DIR}/config

# Set up port-forward to the k83 control endpoint and record the PID of the
# background task.
ssh -N -L 6443:localhost:6443 chrome-bot@100.115.95.135 &
PID=$!

echo ${PID} > ${DIR}/pid

# Start bash.
/bin/bash

# Clean up on exit.
kill ${PID}
rm ${DIR}/config
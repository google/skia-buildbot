#!/bin/bash

# Creates a shell where kubectl is hooked up to the cluster.

if [ $# -ne 1 ]; then
    echo "$0 <rack>"
    exit 1
fi

# TODO have a list that maps rack name to IP address
# maybe each one should use a different port besides 6443?

RACK=$1

CLUSTER=skolo-${RACK}

DIR=${HOME}/.config/skia-infra/skolo/skolo-${RACK}

# Create dir to store config.
mkdir -p ${DIR}

# Grab config from the kubernetes cluster and store in the temp dir.
ssh chrome-bot@100.115.95.135 "sudo kubectl config view --raw" > ${DIR}/config

# Make kubectl use that config.
export KUBECONFIG=${DIR}/config

# Set up port-forward to the k83 control endpoint and record the PID of the
# background task.
ssh -N -L 6442:localhost:6443 chrome-bot@100.115.95.135 &
PID=$!

# Change the name so we can track which cluster we are talking to.
kubectl config rename-context default ${CLUSTER}
kubectl config set-cluster default --server=https://127.0.0.1:6442

echo ${PID} > ${DIR}/pid

# Start bash.
/bin/bash

# Clean up on exit.
kill ${PID}
rm ${DIR}/config
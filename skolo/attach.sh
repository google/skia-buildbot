#!/bin/bash

# Creates a shell where kubectl is hooked up to the RPI2 cluster.

set -e -x

# Create temp dir to store config.
DIR=`mktemp -d`

# Grab config from the kubernetes cluster and store in the temp dir.
ssh chrome-bot@100.115.95.135 "sudo kubectl config view --raw" > ${DIR}/config

# Make kubectl use that config.
export KUBECONFIG=${DIR}/config

# Set up port-forward to the k83 control endpoint and record the PID of the
# background task.
ssh -N -L 6443:localhost:6443 chrome-bot@100.115.95.135 &
PID=$!

# Start bash.
/bin/bash

# Clean up on exit.
kill ${PID}
rm ${DIR}/config
#!/bin/bash

# Creates a shell where kubectl is hooked up to the selected cluster.

CLUSTER=skolo-rack4

# Where we will store the config.
DIR=${HOME}/.config/skia-infra/skolo/${CLUSTER}

# Create dir to store config.
mkdir -p ${DIR}

# Make kubectl use that config.
export KUBECONFIG=${DIR}/config


IP=100.115.95.135
PORT=6442

# Grab config from the kubernetes cluster and store in the config file.
ssh chrome-bot@${IP} "sudo kubectl config view --raw" > ${DIR}/config

# Set up port-forward to the k83 control endpoint and record the PID of the
# background task.
    ssh -N -L ${PORT}:localhost:6443 chrome-bot@${IP} &
    PID=$!

# Wait until the port is available.
until nc -z localhost ${PORT}
do
  sleep 1
  echo "Waiting for port-forward to come up."
done

# Change the name so we can track which cluster we are talking to.
kubectl config rename-context default ${CLUSTER}
kubectl config set-cluster default --server=https://127.0.0.1:${PORT}

echo ${PID} > ${DIR}/pid

# Protect the config file.
chmod 600 ${DIR}/config

echo "Remember to exit this shell to disconnect from the cluster."

/bin/bash

# Clean up on exit.
if [ "${PID}" != "" ]; then
    kill ${PID}
fi
#!/bin/bash

# Creates a shell where kubectl is hooked up to the cluster.

if [ $# -ne 1 ]; then
    echo "$0 <cluster>"
    exit 1
fi

CLUSTER=$1

REL=$(dirname "$0")

# What type of cluster are we connecting to?
TYPE=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".type")

if [ "$TYPE" == "null" ]; then
    echo "$1 is not a valid cluster name."
    exit 1
fi

# Where we will store the config.
DIR=${HOME}/.config/skia-infra/skolo/${CLUSTER}

# Create dir to store config.
mkdir -p ${DIR}

# Make kubectl use that config.
export KUBECONFIG=${DIR}/config

if [ "${TYPE}" == "gke" ]; then
    PROJECT=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".project")
    ZONE=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".zone")
    gcloud container clusters get-credentials ${CLUSTER} --zone ${ZONE} --project ${PROJECT}
    gcloud config set project ${PROJECT}
else # Type == "k3s".
    IP=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".ip")
    PORT=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".port")

    # Grab config from the kubernetes cluster and store in the config file.
    ssh chrome-bot@${IP} "sudo kubectl config view --raw" > ${DIR}/config

    # Set up port-forward to the k83 control endpoint and record the PID of the
    # background task.
    ssh -N -L ${PORT}:localhost:6443 chrome-bot@${IP} &
    PID=$!

    # Change the name so we can track which cluster we are talking to.
    kubectl config rename-context default ${CLUSTER}
    kubectl config set-cluster default --server=https://127.0.0.1:${PORT}

    echo ${PID} > ${DIR}/pid
fi

echo "Remember to exit this shell to disconnect from the cluster."

# Start bash.
/bin/bash

# Clean up on exit.
if [ "${PID}" != "" ]; then
    kill ${PID}
fi
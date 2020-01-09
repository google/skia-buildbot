#!/bin/bash

# Creates a shell where kubectl is hooked up to the selected cluster.

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# -ne 1 ]; then
    echo "$0 <cluster>"
    echo ""
    echo -n "Valid cluster names: "
    cat ${REL}/../kube/clusters/config.json | jq -r ".clusters | keys |  @csv"
    exit 1
fi

CLUSTER=$1

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
    # Since we've set KUBECONFIG at this point the following commands will
    # change that file, not the default one at ~/.kube/config.
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

# Protect the config file.
chmod 600 ${DIR}/config

echo "Remember to exit this shell to disconnect from the cluster."

# Start bash.
/bin/bash

# Clean up on exit.
if [ "${PID}" != "" ]; then
    kill ${PID}
fi
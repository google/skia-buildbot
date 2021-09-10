#!/bin/bash

# Creates a shell where kubectl is hooked up to the selected cluster. Make sure
# you have followed the ssh setup instructions before running this script:
#
#     http://go/skolo-maintenance#heading=h.or4jzu6r2mzn
#

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# == 0 ]; then
    echo "$0 <cluster> [<optional commmand line to run.>]"
    echo ""
    echo -n "Valid cluster names: "
    cat ${REL}/../kube/clusters/config.json | jq -r ".clusters | keys |  @csv"
    exit 1
fi

CLUSTER=$1; shift

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

# Since we've set KUBECONFIG at this point the following commands will
# change that file, not the default one at ~/.kube/config.
PROJECT=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".project")
ZONE=$(cat ${REL}/../kube/clusters/config.json | jq -r ".clusters.\"${CLUSTER}\".zone")
gcloud container clusters get-credentials ${CLUSTER} --zone ${ZONE} --project ${PROJECT}
gcloud config set project ${PROJECT}

# Protect the config file.
chmod 600 ${DIR}/config

if [ $# -ne 0 ] ; then
    printf -v command_line '%q ' "$@"
    /bin/bash -c "$command_line"
else
    echo "Remember to exit this shell to disconnect from the cluster."
    /bin/bash
fi

# Clean up on exit.
if [ "${PID}" != "" ]; then
    kill ${PID}
fi
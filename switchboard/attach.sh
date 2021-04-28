#!/bin/bash

# Creates a port-forward to the device under test.

POD=`curl https://switchboard.skia.org/lease | jq -r  .Pod`
PORT=`curl https://switchboard.skia.org/lease | jq -r  .Port`

# Where we will store the config.
DIR=${HOME}/.config/skia-infra/skolo/skia-switchboard

# Create dir to store config.
mkdir -p ${DIR}

# Make kubectl use that config.
export KUBECONFIG=${DIR}/config

# Since we've set KUBECONFIG at this point the following commands will
# change that file, not the default one at ~/.kube/config.
PROJECT=skia-switchboard
ZONE=us-central1-c
gcloud container clusters get-credentials skia-switchboard --zone ${ZONE} --project ${PROJECT}
gcloud config set project ${PROJECT}

# Protect the config file.
chmod 600 ${DIR}/config

kubectl port-forward ${POD} ${PORT}
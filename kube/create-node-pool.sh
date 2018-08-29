#/bin/bash

# Creates a node-pool following the best security practices at the time.
# Turns off unsafe addons and uses a service account with the minimum
# set of needed permissions to run Kubernetes. See
# https://cloudplatform.googleblog.com/2017/11/precious-cargo-securing-containers-with-Kubernetes-Engine-18.html

# This script presumes the service account has already been created, which is
# done in ./create-cluster.sh, which needs to have been run before this
# script.

set -x -e

source ./config.sh

NODE_POOL=n1-highmem-64

gcloud container node-pools create ${NODE_POOL} \
  --cluster "${CLUSTER_NAME}" \
  --service-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --disk-size "200" \
  --enable-autoscaling \
  --enable-autoupgrade \
  --enable-autorepair \
  --image-type "COS" \
  --machine-type "n1-highmem-64" \
  --min-nodes "1" --max-nodes "15" \
  --num-nodes "1" \
  --zone "us-central1-a"

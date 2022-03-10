#!/bin/bash

# The following variables should be set in the yaml file:
# - KUBECONFIG
# - PORT_ON_THANOS_QUERY
# - K8S_SERVER
# - CLOUDSDK_COMPUTE_ZONE
# - CLOUDSDK_CONTAINER_CLUSTER
# - CLOUDSDK_COMPUTE_REGION

set -xe

echo "Starting bouncer.sh"

# Echo something useful into the logs.
kubectl config get-clusters

# Set up the reverse port-forward.
SERVER_FLAG=""
if [ -n "$K8S_SERVER" ]; then
  SERVER_FLAG="--server=${K8S_SERVER}"
fi
kubectl get pod ${SERVER_FLAG} -l app=thanos-query -o jsonpath="{.items[0].metadata.name}"
nc -vv 127.0.0.1 9001 -c "kubectl exec -i $(kubectl get pod -l app=thanos-query -o jsonpath=\"{.items[0].metadata.name}\") -- nc -vv -l -p ${PORT_ON_THANOS_QUERY}"

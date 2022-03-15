#!/bin/bash

# The following variables should be set in the yaml file:
# - PORT_ON_THANOS_QUERY
# - CLOUDSDK_COMPUTE_REGION
# - CLOUDSDK_COMPUTE_ZONE
# - CLOUDSDK_CONTAINER_CLUSTER
# - CLOUDSDK_CORE_PROJECT

echo "Starting bouncer.sh"

KUBECTL_WRAPPER=/builder/kubectl.bash

# Echo something useful into the logs. Use the wrapper to set up authentication.
${KUBECTL_WRAPPER} config get-clusters
${KUBECTL_WRAPPER} config view --raw

# Set up the reverse port-forward.
kubectl get pod --namespace=default -l app=thanos-query -o jsonpath="{.items[0].metadata.name}"
nc -vv 127.0.0.1 9001 -c "kubectl exec -i --namespace=default $(kubectl get pod --namespace=default -l app=thanos-query -o jsonpath=\"{.items[0].metadata.name}\") -- nc -vv -l -p ${PORT_ON_THANOS_QUERY}"

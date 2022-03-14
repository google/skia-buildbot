#!/bin/bash

# The following variables should be set in the yaml file:
# - KUBECONFIG
# - PORT_ON_THANOS_QUERY
# - CLOUDSDK_COMPUTE_REGION
# - CLOUDSDK_COMPUTE_ZONE
# - CLOUDSDK_CONTAINER_CLUSTER
# - CLOUDSDK_CORE_PROJECT

echo "Starting bouncer.sh"

KUBECTL=/builder/kubectl.bash

# Echo something useful into the logs.
${KUBECTL} config get-clusters
${KUBECTL} config view --raw

# Set up the reverse port-forward.
${KUBECTL} get pod -l app=thanos-query -o jsonpath="{.items[0].metadata.name}"
nc -vv 127.0.0.1 9001 -c "${KUBECTL} exec -i $(${KUBECTL} get pod -l app=thanos-query -o jsonpath=\"{.items[0].metadata.name}\") -- nc -vv -l -p ${PORT_ON_THANOS_QUERY}"

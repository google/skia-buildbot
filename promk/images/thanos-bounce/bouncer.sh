#!/bin/bash
echo "Starting bouncer.sh"

# KUBECONFIG should be set in yaml file.
# PORT_ON_THANOS_QUERY should be set in the yaml file.

# Echo something useful into the logs.
kubectl config get-clusters

# Set up the reverse port-forward.
kubectl get pod -l app=thanos-query -o jsonpath="{.items[0].metadata.name}"
nc -vv 127.0.0.1 9001 -c "kubectl exec -i $(kubectl get pod -l app=thanos-query -o jsonpath=\"{.items[0].metadata.name}\") -- nc -vv -l -p ${PORT_ON_THANOS_QUERY}"

#!/bin/bash
echo "Starting bouncer.sh"
# KUBECONFIG should be set in yaml file.
kubectl config get-clusters
kubectl get pod -l app=thanos-query -o jsonpath="{.items[0].metadata.name}"
nc -vv 127.0.0.1 9001 -c "kubectl exec -i $(kubectl get pod -l app=thanos-query -o jsonpath=\"{.items[0].metadata.name}\") -- nc -vv -l -p 9002"
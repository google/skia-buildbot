#/bin/bash

# Copies the service account for alert-to-pubsub to skia-corp.

set -e -x
source ../bash/ramdisk.sh
source ../kube/clusters.sh

cd /tmp/ramdisk

__skia_public
kubectl get secret skia-alert-to-pubsub --output=yaml > secret.yaml
__skia_corp
kubectl apply -f secret.yaml

cd -

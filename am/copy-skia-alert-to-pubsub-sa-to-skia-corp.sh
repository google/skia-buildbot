#/bin/bash

# Copies the service account for alert-to-pubsub to skia-corp.

set -e -x
source ../bash/ramdisk.sh

cd /tmp/ramdisk

gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public
kubectl get secret skia-alert-to-pubsub --output=yaml > secret.yaml
gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp
kubectl apply -f secret.yaml

cd -

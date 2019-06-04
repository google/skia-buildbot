#/bin/bash

# Creates the service account key used by metadata-server, and export a key for
# it into the kubernetes cluster as a secret.

set -e -x
source ../kube/clusters.sh
source ../bash/ramdisk.sh

SA_NAME="skolo-service-accounts"

__skia_rpi2

cd /tmp/ramdisk

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="chrome-swarming-bots@skia-buildbots.google.com.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

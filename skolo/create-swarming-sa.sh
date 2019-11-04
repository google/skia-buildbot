#/bin/bash

# Creates the service account key used by metadata-server, and export a key for
# it into the kubernetes cluster as a secret.

set -e -x
source ../bash/ramdisk.sh

SA_NAME="skolo-service-accounts"

cd /tmp/ramdisk

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com" --project=skia-public

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

#/bin/bash

# Creates the service account used by the power-controller server, and export a key for
# it into the kubernetes cluster as a secret.
# Afterwards, the steps on https://bugs.chromium.org/p/skia/issues/detail?id=8248#c5 are
# needed to hook the service account up to swarming (don't need task-scheduling
# permissions, just viewing ones).

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="skia-power-controller"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for Skia Power Controller"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

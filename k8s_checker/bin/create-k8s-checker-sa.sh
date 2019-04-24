#/bin/bash

# Creates the service account used by K8s Checker, and export a key for
# it into the kubernetes cluster as a secret.

set -e -x
source ../../kube/config.sh
source ../../bash/ramdisk.sh

# New service account we will create.
SA_NAME="skia-k8s-checker"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for K8s Checker"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

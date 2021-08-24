#/bin/bash

# Creates the service account used by AutoRoll Backend, and export a key for it
# into the kubernetes cluster as a secret.

set -e -x
source ../../kube/corp-config.sh
source ../../bash/ramdisk.sh

# New service account we will create.
SA_NAME="android-auto-approver"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for AutoRolls into Android"
gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_EMAIL}"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

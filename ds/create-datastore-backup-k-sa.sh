#/bin/bash

# Creates the service account used by datastore_backup_k, and export a key for
# it into the kubernetes cluster as a secret.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="skia-datastore-backup-k"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for datastore-backup-k"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/datastore.owner

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

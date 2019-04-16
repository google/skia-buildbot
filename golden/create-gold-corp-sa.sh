#/bin/bash

# Creates the service account used by Gold running in K8s and export a key for
# it into the kubernetes cluster as a secret.

set -e -x
source ../kube/corp-config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="skia-gold"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" \
    --display-name="Service account for Skia Gold in skia-corp"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/bigtable.user

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/datastore.user

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/pubsub.admin

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/storage.admin

gcloud beta iam service-accounts keys create ${SA_NAME}.json \
    --iam-account="${SA_EMAIL}"

set +e
kubectl delete secret "${SA_NAME}"
set -e

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

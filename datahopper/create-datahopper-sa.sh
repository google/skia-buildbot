#/bin/bash

# Creates the service account used by Datahopper, and export a key for it
# into the kubernetes cluster as a secret.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="datahopper"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for Skia Datahopper"
gcloud projects add-iam-policy-binding skia-firestore --member serviceAccount:${SA_EMAIL} --role roles/datastore.user
gcloud projects add-iam-policy-binding skia-firestore --member serviceAccount:${SA_EMAIL} --role roles/datastore.importExportAdmin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member serviceAccount:${SA_EMAIL} --role roles/pubsub.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member serviceAccount:${SA_EMAIL} --role roles/bigtable.user
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member serviceAccount:${SA_EMAIL} --role roles/monitoring.viewer
gcloud projects add-iam-policy-binding skia-swarming-bots --member serviceAccount:${SA_EMAIL} --role roles/monitoring.viewer
gsutil acl ch -u "${SA_EMAIL}:W" gs://skia-perf

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_EMAIL}"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -

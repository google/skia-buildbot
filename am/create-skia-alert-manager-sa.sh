#/bin/bash

# Creates the service account for alert-manager.

set -e -x
source ../kube/config.sh

# New service account we will create.
SA_NAME=skia-alert-manager

gcloud iam service-accounts create "${SA_NAME}" --display-name="alert-manager service account"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/datastore.user

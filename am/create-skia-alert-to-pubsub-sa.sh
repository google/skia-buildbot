#/bin/bash

# Creates the service account for alert-to-pubsub.

set -e -x
source ../kube/config.sh

# New service account we will create. Name must be the same for all clusters
# that alert-to-pubsub runs in.
SA_NAME=skia-alert-to-pubsub

gcloud iam service-accounts create "${SA_NAME}" --display-name="alert-to-pubsub service account."

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/logging.logWriter
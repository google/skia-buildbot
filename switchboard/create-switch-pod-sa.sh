#/bin/bash

# Creates the service account for switch-pod.

set -e -x
source ../kube/switchboard-config.sh

# New service account we will create.
SA_NAME=switch-pod

# Create service account
gcloud iam service-accounts create "${SA_NAME}" --display-name="switch-pod service account"

# Allow k8s service account to impersonate the GCP service account.
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:${PROJECT_SUBDOMAIN}.svc.id.goog[default/${SA_NAME}]" \
  ${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com

# Allow access to FireStore, which uses the datastore.user role.
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/datastore.user


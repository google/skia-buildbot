#/bin/bash
# Creates the service account for perf.

set -e -x

source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=skia-perf

cd /tmp/ramdisk
gcloud iam service-accounts create "${SA_NAME}" --display-name="perf-ingest service account"

gcloud projects add-iam-policy-binding --project ${PROJECT} \
  --member serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com \
  --role roles/cloudtrace.agent

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor

# Allow the new service account to impersonate the Google Cloud service account. See
# https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#authenticating_to.
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:skia-public.svc.id.goog[default/skia-perf]" \
  skia-perf@skia-public.iam.gserviceaccount.com

gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://skia-perf

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"
kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json
cd -

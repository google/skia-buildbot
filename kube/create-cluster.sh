#/bin/bash

# Creates a cluster following the best security practices at the time.
# Turns off unsafe addons and uses a service account with the minimum
# set of needed permissions to run Kubernetes. See
# https://cloudplatform.googleblog.com/2017/11/precious-cargo-securing-containers-with-Kubernetes-Engine-18.html

set -x -e

source ./config.sh

# New service account we will create.
SA_NAME=skia-public-k8s

gcloud iam service-accounts create "${SA_NAME}" \
  --display-name="${SA_NAME}"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role roles/logging.logWriter

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role roles/monitoring.metricWriter

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role roles/monitoring.viewer

gcloud container clusters create "${CLUSTER_NAME}" \
  --service-account="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --addons HorizontalPodAutoscaling,HttpLoadBalancing \
  --cluster-version "1.9.6-gke.1" \
  --disk-size "100" \
  --enable-autoscaling \
  --enable-autoupgrade \
  --enable-autoupgrade \
  --enable-cloud-logging \
  --enable-cloud-monitoring \
  --image-type "COS" \
  --machine-type "n1-standard-8" \
  --maintenance-window "07:00" \
  --min-nodes "3" --max-nodes "100" \
  --network "default" \
  --no-enable-basic-auth \
  --no-enable-legacy-authorization \
  --num-nodes "3" \
  --subnetwork "default" \
  --zone "us-central1-a"

# Add service account as reader of docker images bucket.
# First remove the account so the add is fresh.
gsutil iam ch -d "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com:objectViewer" gs://artifacts.skia-public.appspot.com
gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com:objectViewer" gs://artifacts.skia-public.appspot.com

echo "Remember to create secrets and push configs for each application."

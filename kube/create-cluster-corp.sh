#/bin/bash

# Creates a cluster following the best security practices at the time.
# Turns off unsafe addons and uses a service account with the minimum
# set of needed permissions to run Kubernetes. See
# https://cloudplatform.googleblog.com/2017/11/precious-cargo-securing-containers-with-Kubernetes-Engine-18.html

set -x -e

source ./corp-config.sh

# gcloud iam service-accounts create "${SA_NAME}" \
#  --project="${PROJECT_ID}" \
#   --display-name="${SA_NAME}"
# 
# gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
#   --project="${PROJECT_ID}" \
#   --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
#   --role roles/logging.logWriter
# 
# gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
#   --project="${PROJECT_ID}" \
#   --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
#   --role roles/monitoring.metricWriter
# 
# gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
#   --project="${PROJECT_ID}" \
#   --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
#   --role roles/monitoring.viewer
# 
# gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
#   --project="${PROJECT_ID}" \
#   --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
#   --role roles/compute.serviceAgent
# 
# gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
#   --project="${PROJECT_ID}" \
#   --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
#   --role roles/storage.objectViewer

gcloud container clusters create "${CLUSTER_NAME}" \
  --project="${PROJECT_ID}" \
  --service-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --addons HorizontalPodAutoscaling,HttpLoadBalancing \
  --disk-size "100" \
  --enable-autoupgrade \
  --enable-cloud-logging \
  --enable-cloud-monitoring \
  --image-type "COS" \
  --machine-type "n1-standard-16" \
  --maintenance-window "07:00" \
  --network "default" \
  --no-enable-basic-auth \
  --no-enable-legacy-authorization \
  --enable-network-policy \
  --num-nodes "3" \
  --subnetwork "default" \
  --network "projects/google.com:skia-corp/global/networks/default"


# Add service account as reader of docker images bucket.
# First remove the account so the add is fresh.
gsutil iam ch -d "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://artifacts.skia-public.appspot.com
gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://artifacts.skia-public.appspot.com

echo "Remember to create secrets and push configs for each application."

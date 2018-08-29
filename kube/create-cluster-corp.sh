#/bin/bash

# Creates a cluster following the best security practices at the time.
# Turns off unsafe addons and uses a service account with the minimum
# set of needed permissions to run Kubernetes. See
# https://cloudplatform.googleblog.com/2017/11/precious-cargo-securing-containers-with-Kubernetes-Engine-18.html

set -x -e

source ./corp-config.sh
source ../bash/ramdisk.sh
source ../kube/clusters.sh

gcloud iam service-accounts create "${SA_NAME}" \
  --display-name="${SA_NAME}"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/logging.logWriter

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/monitoring.metricWriter

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/monitoring.viewer

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/compute.serviceAgent

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/storage.objectViewer

gcloud container clusters create "${CLUSTER_NAME}" \
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

##################################################################
#
# Add the ability for the new cluster to pull docker images from
# gcr.io/skia-public container registry.
#
##################################################################

# Add service account as reader of docker images bucket.
# First remove the account so the add is fresh.
gsutil iam ch -d "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://artifacts.skia-public.appspot.com
gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://artifacts.skia-public.appspot.com

# The following articles explain what is happening in the rest of this section:
#
# https://medium.com/google-cloud/using-googles-private-container-registry-with-docker-1b470cf3f50a
# https://kubernetes.io/docs/concepts/containers/images/#using-a-private-registry
# https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account

cd /tmp/ramdisk

# Download a key for the clusters default service account.
gcloud beta iam service-accounts keys create ${SA_NAME}.json \
  --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

# Use that key as a docker-registry secret.
kubectl create secret docker-registry "${SA_NAME}" \
 --docker-username=_json_key \
 --docker-password="`cat ${SA_NAME}.json`" \
 --docker-server=https://gcr.io \
 --docker-email=skiabot@google.com

# Modify the default service account so that it always uses the above secret when pulling images.
kubectl patch serviceaccount default -p "{\"imagePullSecrets\": [{\"name\": \"${SA_NAME}\"}]}"

cd -

# Show that the modification has worked.
kubectl get secrets

kubectl get serviceaccounts default -o json

echo "Remember to create secrets and push configs for each application."

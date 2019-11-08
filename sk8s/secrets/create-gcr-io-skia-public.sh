#/bin/bash

set -x -e

source $(dirname "$0")/../../kube/config-only.sh
source $(dirname "$0")/../../bash/ramdisk.sh

SA_NAME=gcr-io-skia-public

CLUSTER=$(kubectl config current-context)
if [ "$CLUSTER" != "skolo_rpi2" ]
then
  echo "Wrong cluster, must be run in skolo_rpi2."
  exit 1
fi

gcloud config set project skia-public
gcloud iam service-accounts create "${SA_NAME}" \
  --display-name="${SA_NAME}"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/storage.objectViewer

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

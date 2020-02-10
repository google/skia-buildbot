#/bin/bash

# Create a service account that can read from the gcr.io/skia-public container
# registry and add it as a docker-registry secret to the cluster.

set -x -e

source ../../bash/ramdisk.sh

SA_EMAIL=$(../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rack4 \
  gcr-io-skia-public-account \
  "cluster service account to access gcr.io/skia-public images" \
  roles/storage.objectViewer)

cd /tmp/ramdisk

# Download a key for the clusters default service account.
gcloud beta iam service-accounts keys create key.json \
  --iam-account="${SA_EMAIL}"

# Use that key as a docker-registry secret.
kubectl create secret docker-registry gcr-io-skia-public \
 --docker-username=_json_key \
 --docker-password="`cat key.json`" \
 --docker-server=https://gcr.io \
 --docker-email=skiabot@google.com

##################################################################
#
# Add the ability for the new cluster to pull docker images from
# gcr.io/skia-public container registry.
#
##################################################################
kubectl patch serviceaccount default -p "{\"imagePullSecrets\": [{\"name\": \"gcr-io-skia-public\"}]}"

# Add service account as reader of docker images bucket.
# First remove the account so the add is fresh.
gsutil iam ch -d "serviceAccount:${SA_EMAIL}:objectViewer" gs://artifacts.skia-public.appspot.com
gsutil iam ch "serviceAccount:${SA_EMAIL}:objectViewer" gs://artifacts.skia-public.appspot.com
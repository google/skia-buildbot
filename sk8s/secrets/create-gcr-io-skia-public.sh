#/bin/bash

set -x -e

SA_EMAIL=$(../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rpi2-01 \
  gcr-io-skia-public \
  "cluster service account to access gcr.io/skia-public images"
  roles/storage.objectViewer)

##################################################################
#
# Add the ability for the new cluster to pull docker images from
# gcr.io/skia-public container registry.
#
##################################################################

# Add service account as reader of docker images bucket.
# First remove the account so the add is fresh.
gsutil iam ch -d "serviceAccount:${SA_EMAIL}:objectViewer" gs://artifacts.skia-public.appspot.com
gsutil iam ch "serviceAccount:${SA_EMAIL}:objectViewer" gs://artifacts.skia-public.appspot.com
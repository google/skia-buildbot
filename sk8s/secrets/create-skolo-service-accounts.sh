#/bin/bash

# Creates the service account key used by metadata-server, and export a key for
# it into the kubernetes cluster as a secret.

set -e -x

../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rack4 \
  skolo-service-accounts \
  "cluster service account to access gcr.io/skia-public images"

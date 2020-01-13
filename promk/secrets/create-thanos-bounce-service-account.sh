#/bin/bash

# Creates the service account key used by the thanos-bounce image.

set -e -x

../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rack4 \
  thanos-bounce-service-account \
  "Service account for headless authentication to the skia-public k8s cluster." \
  roles/container.developer

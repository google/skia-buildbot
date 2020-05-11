#/bin/bash

# Creates the service account key used by podwatcher.

set -e -x

../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rack4 \
  podwatcher-service-account \
  "cluster service account for bots running in the skolo" \
  roles/logging.logWriter \
  roles/datastore.user
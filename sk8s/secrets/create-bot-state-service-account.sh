#/bin/bash

# Creates the service account key used by all bots, and export a key for
# it into the kubernetes cluster as a secret.

set -e -x

../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rack4-shelf1 \
  skolo-bot-service-account \
  "cluster service account for bots running in the skolo" \
  roles/logging.logWriter \
  roles/pubsub.admin \
  roles/datastore.user
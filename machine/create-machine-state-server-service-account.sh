#/bin/bash

# Creates the service account for machines.skia.org.
../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  machine-state-server \
  "The machine state server service account." \
  roles/pubsub.admin \
  roles/datastore.user
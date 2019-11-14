#/bin/bash

# Creates the service account for alert-to-pubsub in skolo-rpi2-01.
../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rpi2-01 \
  skolo-rpi2-alert-to-pubsub \
  "The alert-to-pubsub service account." \
  roles/pubsub.editor
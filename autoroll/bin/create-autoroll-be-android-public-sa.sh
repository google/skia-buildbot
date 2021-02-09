#/bin/bash

# Creates the service account for bugs central.

../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  android-autoroll \
  "Service account for AutoRolls into Android"

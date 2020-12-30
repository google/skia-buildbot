#/bin/bash

# Creates the service account for scrap.skia.org.
../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  scrapexchange \
  "The scrapexchange server service account." \
  roles/storage.objectAdmin
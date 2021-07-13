#/bin/bash

# Creates the service account for SkCQ Backend.

../../kube/secrets/add-service-account.sh \
  google.com:skia-corp \
  skia-corp \
  skcq-be \
  "Service account for SkCQ Backend."

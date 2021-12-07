#/bin/bash

# Creates the service account used by SA Keys Checker, and export a key for
# it into the kubernetes cluster as a secret.

../../kube/secrets/add-service-account.sh \
  google.com:skia-corp \
  skia-corp \
  skia-sa-keys-checker \
  "Service account for SA Keys Checker."

../../kube/apply-secret-to-cluster.sh \
  skia-corp \
  skia-sa-keys-checker

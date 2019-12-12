#/bin/bash

# Creates the service account used by K8s Checker, and export a key for
# it into the kubernetes cluster as a secret.

../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-k8s-checker \
  "Service account for K8s Checker for access to Gerrit."

  ../kube/secrets/add-service-account.sh \
  skia-public \
  skia-corp \
  skia-k8s-checker \
  "Service account for K8s Checker for access to Gerrit."
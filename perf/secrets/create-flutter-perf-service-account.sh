#/bin/bash

# Creates the service account for flutter perf.
../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  flutter-perf-service-account \
  "The flutter perf service account." \
  roles/pubsub.editor \
  roles/cloudtrace.agent
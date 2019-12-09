#/bin/bash

# Creates the service account used by the docker_pushes_watcher server, and
# export a key for it into the kubernetes cluster as a secret.
../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-docker-pushes-watcher \
  "The docker_pushes_watcher service account." \
  roles/datastore.user

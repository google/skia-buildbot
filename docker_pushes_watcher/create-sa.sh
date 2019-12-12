#/bin/bash

# Creates the service account used by the docker_pushes_watcher server.
../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-docker-pushes-watcher \
  "The docker_pushes_watcher service account." \
  roles/datastore.user

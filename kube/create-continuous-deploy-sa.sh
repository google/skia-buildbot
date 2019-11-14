#/bin/bash

# Create the service account that has access to the skia-public-config repo
# and export a key for it into the kubernetes cluster as a secret.

set -e -x

./secrets/add-service-account.sh skia-public skia-public skia-continuous-deploy "Continous deploy to skia-public."
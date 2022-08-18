#!/bin/bash

set -e

../kube/secrets/rotate-keys-for-skia-corp-sa.sh google.com:skia-corp skia-eskia deployment/eskia-api

# Since two deployments use the same service account we need to restart eskia-coverage separately.
../kube/attach.sh skia-corp kubectl rollout restart deployment/eskia-coverage
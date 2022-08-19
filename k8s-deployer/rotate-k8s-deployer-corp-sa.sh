#!/bin/bash

set -e

../kube/secrets/rotate-keys-for-skia-corp-sa.sh google.com:skia-corp k8s-deployer deployment/k8s-deployer
# Note: we aren't rotating the key for the skia-public account because we can
# use workload identity in that cluster and the key is therefore unused.

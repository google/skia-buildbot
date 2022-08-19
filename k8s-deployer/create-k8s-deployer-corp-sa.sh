#!/bin/bash

set -e

../kube/secrets/add-service-account.sh google.com:skia-corp skia-corp k8s-deployer "Service account for k8s-deployer"
# Note: We can use workload identity in skia-public, so the key created by this
# script will not be used.
../kube/secrets/add-service-account.sh skia-public skia-public k8s-deployer "Service account for k8s-deployer"

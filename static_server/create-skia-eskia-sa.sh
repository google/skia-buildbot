#!/bin/bash

set -e

../kube/secrets/add-service-account.sh google.com:skia-corp skia-corp skia-eskia "Service account for eskia apps"

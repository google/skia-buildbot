#!/bin/bash

set -e

../kube/secrets/rotate-keys-for-skia-corp-sa.sh google.com:skia-corp alert-to-pubsub deployment/alert-to-pubsub

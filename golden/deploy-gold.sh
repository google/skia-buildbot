#!/bin/bash

set -x

TMPL_DIR=./k8s-config-templates
INSTANCE_ID=fuchsia
# INSTANCE_ID=$1

kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${TMPL_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/gold-k8s-template.yaml" \
              -o "gold-${INSTANCE_ID}.yaml" \

echo "Instance ${INSTANCE_ID} generated."

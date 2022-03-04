#!/bin/bash

# Generate the configmaps used for Prometheus and Thanos.

set -e -x

# Setup.
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
OUT_DIR="${1:-$(pwd)}"
echo "writing output to ${OUT_DIR}"
mkdir -p ${OUT_DIR}/skia-public
mkdir -p ${OUT_DIR}/skia-corp
mkdir -p ${OUT_DIR}/skia-infra-public-dev

# skia-public/thanos-rules.yml
TMP_DIR="$(mktemp -d)"
cp ${SCRIPT_DIR}/prometheus/alerts_*.yml ${TMP_DIR}
cp ${SCRIPT_DIR}/prometheus/absent_alerts_*.yml ${TMP_DIR}
cp ${SCRIPT_DIR}/prometheus/*_rules.yml ${TMP_DIR}
kubectl create configmap thanos-rules --from-file=${TMP_DIR} -o yaml --dry-run=client > ${OUT_DIR}/skia-public/thanos-rules.yml

# skia-public/prometheus-server-conf.yml
TMP_DIR="$(mktemp -d)"
cp ${SCRIPT_DIR}/prometheus/prometheus-public.yml ${TMP_DIR}/prometheus.yml
cp ${SCRIPT_DIR}/prometheus/alerts_thanos.yml ${TMP_DIR}
cp ${SCRIPT_DIR}/prometheus/absent_alerts_thanos.yml ${TMP_DIR}
kubectl create configmap prometheus-server-conf --from-file=${TMP_DIR} -o yaml --dry-run=client > ${OUT_DIR}/skia-public/prometheus-server-conf.yml

# skia-corp/prometheus-server-conf.yml
TMP_DIR="$(mktemp -d)"
cp ${SCRIPT_DIR}/prometheus/prometheus-corp.yml ${TMP_DIR}/prometheus.yml
kubectl create configmap prometheus-server-conf --from-file=${TMP_DIR} -o yaml --dry-run=client > ${OUT_DIR}/skia-corp/prometheus-server-conf.yml

# skia-infra-public-dev/prometheus-server-conf.yml
TMP_DIR="$(mktemp -d)"
cp ${SCRIPT_DIR}/prometheus/prometheus-skia-infra-public-dev.yml ${TMP_DIR}/prometheus.yml
kubectl create configmap prometheus-server-conf --from-file=${TMP_DIR} -o yaml --dry-run=client > ${OUT_DIR}/skia-infra-public-dev/prometheus-server-conf.yml

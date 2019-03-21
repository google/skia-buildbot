#!/bin/bash

set -x -e

INSTANCE_ID=fuchsia
TMPL_DIR=./k8s-config-templates
INSTANCE_DIR=./k8s-instances

CONF_OUT_DIR="./build"
CONF_MAP="gold-${INSTANCE_ID}-ingestion-config"
# DEPLOY_CONF="${CONF_OUT_DIR}/gold-${INSTANCE_ID}.yaml"
DEPLOY_CONF="${CONF_OUT_DIR}/gold-${INSTANCE_ID}"
INGEST_CONF="${CONF_OUT_DIR}/${CONF_MAP}.json5"

TRACE_SERVER_CONF="${DEPLOY_CONF}-traceserver.yaml"
INGESTION_SERVER_CONF="${DEPLOY_CONF}-ingestion.yaml"
CORRECTNESS_CONF="${DEPLOY_CONF}-skiacorrectness.yaml"
BASELINE_SERVER_CONF="${DEPLOY_CONF}-baselineserver.yaml"

mkdir -p $CONF_OUT_DIR
rm -f $CONF_OUT_DIR/*
rm -f $INGEST_CONF

# generate the deployment file for the trace server.
kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${INSTANCE_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/gold-traceserver-template.yaml" \
              -o "${TRACE_SERVER_CONF}"

# generate the configuration file for ingestion.
kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${INSTANCE_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/ingest-config-template.json5" \
              -o "${INGEST_CONF}"

# generate the deployment file for ingestion.
kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${INSTANCE_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/gold-ingestion-template.yaml" \
              -o "${INGESTION_SERVER_CONF}"

# generate the deployment file for skiacorrectness (the main Gold process)
kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${INSTANCE_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/gold-skiacorrectness-template.yaml" \
              -o "${CORRECTNESS_CONF}"

# generate the deployment file for the baseline server
kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${INSTANCE_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/gold-baselineserver-template.yaml" \
              -o "${BASELINE_SERVER_CONF}"

# Push the config map to kubernetes
set +x
echo "# To push these run:\n"
echo "kubectl delete configmap $CONF_MAP"
echo "kubectl create configmap $CONF_MAP --from-file=$INGEST_CONF"

# Push the trace server and show pods so we can see if it landed correctly.
echo "kubectl apply -f ${TRACE_SERVER_CONF}"
echo "kubectl get pods -w -l app=gold-$INSTANCE_ID-traceserver"

# Push the trace server and show pods so we can see if it landed correctly.
echo "kubectl apply -f ${INGESTION_SERVER_CONF}"
echo "kubectl get pods -w -l app=gold-$INSTANCE_ID-ingestion"


# Push the trace server and show pods so we can see if it landed correctly.
echo "kubectl apply -f ${CORRECTNESS_CONF}"
echo "kubectl get pods -w -l app=gold-$INSTANCE_ID-skiacorrectness"

# Push the trace server and show pods so we can see if it landed correctly.
echo "kubectl apply -f ${BASELINE_SERVER_CONF}"
echo "kubectl get pods -w -l app=gold-$INSTANCE_ID-baselineserver"

# Push the trace server and show pods so we can see if it landed correctly.
echo "Instance ${INSTANCE_ID} generated."

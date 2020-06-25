#!/bin/bash

print_usage() {
    echo "Usage: $0 INSTANCE_ID"
    echo "       INSTANCE_ID is the id of the instance to be generated."
    exit 1
}
if [ "$#" -ne 1 ]; then
    print_usage
fi

INSTANCE_ID=$1
set -x -e

TMPL_DIR=./k8s-config-templates
INSTANCE_DIR=./k8s-instances

CONF_OUT_DIR="./build"
ING_CONF_MAP="gold-${INSTANCE_ID}-ingestion-config-bt"
DEPLOY_CONF="${CONF_OUT_DIR}/gold-${INSTANCE_ID}"

INGESTION_SERVER_CONF="${DEPLOY_CONF}-ingestion-bt.yaml"
CORRECTNESS_CONF="${DEPLOY_CONF}-skiacorrectness.yaml"
BASELINE_SERVER_CONF="${DEPLOY_CONF}-baselineserver.yaml"
DIFF_SERVER_CONF="${DEPLOY_CONF}-diffserver.yaml"

mkdir -p $CONF_OUT_DIR
rm -f $CONF_OUT_DIR/*

# Make sure we have the latest and greatest kube-conf-gen
go install ../kube/go/kube-conf-gen

# generate the deployment file for skiacorrectness (the main Gold process)
kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
              -c "${INSTANCE_DIR}/${INSTANCE_ID}-instance.json5" \
              -extra "INSTANCE_ID:${INSTANCE_ID}" \
              -t "${TMPL_DIR}/gold-skiacorrectness-template.yaml" \
              -parse_conf=false -strict \
              -o "${CORRECTNESS_CONF}"

if [ $INSTANCE_ID != "skia-public" ]
then
  # generate the deployment file for ingestion.
  kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
                -c "${INSTANCE_DIR}/${INSTANCE_ID}/${INSTANCE_ID}.json5" \
                -c "${INSTANCE_DIR}/${INSTANCE_ID}/${INSTANCE_ID}-ingestion-bt.json5" \
                -extra "INSTANCE_ID:${INSTANCE_ID}" \
                -t "${TMPL_DIR}/gold-ingestion-bt-template.yaml" \
                -parse_conf=false -quote -strict \
                -o "${INGESTION_SERVER_CONF}"

  # generate the deployment file for the baseline server
  kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
                -c "${INSTANCE_DIR}/${INSTANCE_ID}/${INSTANCE_ID}.json5" \
                -c "${INSTANCE_DIR}/${INSTANCE_ID}/${INSTANCE_ID}-baselineserver.json5" \
                -extra "INSTANCE_ID:${INSTANCE_ID}" \
                -t "${TMPL_DIR}/gold-baselineserver-template.yaml" \
                -parse_conf=false -quote -strict \
                -o "${BASELINE_SERVER_CONF}"

  kube-conf-gen -c "${TMPL_DIR}/gold-common.json5" \
                -c "${INSTANCE_DIR}/${INSTANCE_ID}/${INSTANCE_ID}.json5" \
                -c "${INSTANCE_DIR}/${INSTANCE_ID}/${INSTANCE_ID}-diffserver.json5" \
                -extra "INSTANCE_ID:${INSTANCE_ID}" \
                -t "${TMPL_DIR}/gold-diffserver-template.yaml" \
                -parse_conf=false -quote -strict \
                -o "${DIFF_SERVER_CONF}"
fi

set +x

if [ $INSTANCE_ID != "skia-public" ]
then
  # Push the ingestion config map to kubernetes
  echo "# To push these run:\n"

  # Push the ingestion and show pods so we can see if it landed correctly.
  echo "kubectl apply -f ${INGESTION_SERVER_CONF} && kubectl get pods -w -l app=gold-$INSTANCE_ID-ingestion-bt"

  # Push the diff server and show pods so we can see if it landed correctly.
  echo "kubectl apply -f ${DIFF_SERVER_CONF} && kubectl get pods -w -l app=gold-$INSTANCE_ID-diffserver"

  # Push the main server and show pods so we can see if it landed correctly.
  echo "kubectl apply -f ${CORRECTNESS_CONF} && kubectl get pods -w -l app=gold-$INSTANCE_ID-skiacorrectness"

  # Push the baseline server and show pods so we can see if it landed correctly.
  echo "kubectl apply -f ${BASELINE_SERVER_CONF} && kubectl get pods -w -l app=gold-$INSTANCE_ID-baselineserver"

  # Push the trace server and show pods so we can see if it landed correctly.
  echo "Instance ${INSTANCE_ID} generated."
else
 echo "# To push these run:\n"
  echo "kubectl delete configmap skia-public-authorized-params"
  echo "kubectl create configmap skia-public-authorized-params --from-file=./k8s-instances/skia-public/authorized-params.json5"

 # Push the main server and show pods so we can see if it landed correctly.
  echo "kubectl apply -f ${CORRECTNESS_CONF} && kubectl get pods -w -l app=gold-$INSTANCE_ID-skiacorrectness"
fi


#!/bin/bash
set -x -e
# Creates a port-forward to the device under test.

# Emulate getting the lease info from our switchboard.skia.org stub.
#
# TODO(jcgregorio) Emulate auth using GOOGLE_APPLICATION_CREDENTIALS?
#POD=`curl https://switchboard.skia.org/lease | jq -r  .Pod`
#PORT=`curl https://switchboard.skia.org/lease | jq -r  .Port`

POD="switch-pod-1"
PORT="9000"


# Make kubectl use that config.
#
# TODO(jcgregorio) Do not use a hard-coded value here.
export KUBECONFIG=switchboard/kubeconfig.yaml

kubectl get pods

# Relies on GOOGLE_APPLICATION_CREDENTIALS pointing to a SA that can access the cluster.
kubectl --logtostderr  port-forward ${POD} ${PORT} &

# Wait until the port is available.
echo "Waiting for port-forward to come up."
until nc -z localhost ${PORT}
do
    sleep 1
    echo "Waiting for port-forward to come up."
done

PID=$!

function cleanup {
if [ "${PID}" != "" ]; then
    kill ${PID}
fi
}

trap cleanup EXIT

touch ${ENV_READY_FILE}

while true
do
    echo "Waiting"
    sleep 3
done
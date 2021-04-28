#!/bin/bash
set -x -e
# Creates a port-forward to the device under test.

POD=`curl https://switchboard.skia.org/lease | jq -r  .Pod`
PORT=`curl https://switchboard.skia.org/lease | jq -r  .Port`

# Where we will store the config.
TMPFILE=mktemp

# Make kubectl use that config.
export KUBECONFIG=${TMPFILE}

# Since we've set KUBECONFIG at this point the following commands will
# change that file, not the default one at ~/.kube/config.


PROJECT=skia-switchboard
ZONE=us-central1-c
gcloud container clusters get-credentials skia-switchboard --zone ${ZONE} --project ${PROJECT}
gcloud config set project ${PROJECT}

kubectl port-forward ${POD} ${PORT} &

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

sleep 3

touch ${ENV_READY_FILE}

# Now using that port-forwarded ssh port, we can port forward adb from the
# remote machine.
ssh -N -p ${PORT} -L 10000:127.0.0.1:${PORT} root@127.0.0.1

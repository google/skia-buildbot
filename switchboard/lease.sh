#!/bin/bash
set -x -e
# Creates a port-forward to the device under test.

POD=`curl https://switchboard.skia.org/lease | jq -r  .Pod`
PORT=`curl https://switchboard.skia.org/lease | jq -r  .Port`

# Where we will store the config.
TMPFILE=mktemp

ls -alR .

# Make kubectl use that config.
export KUBECONFIG=switchboard/kubeconfig.yaml

# Relies on GOOGLE_APPLICATION_CREDENTIALS pointing to a SA that can access the cluster.
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

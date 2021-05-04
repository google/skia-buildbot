#!/bin/bash
set -x -e
# Creates a port-forward to the device under test.

# Now using that port-forwarded ssh port, we can port forward adb from the
# remote machine.
ssh -N -L 10000:127.0.0.1:9000 root@switchboard-target-01 &

PID=$!

function cleanup {
if [ "${PID}" != "" ]; then
    kill ${PID}
fi
}

trap cleanup EXIT

adb -H 127.0.0.1 -P 10000 wait-for-any-device

touch ${ENV_READY_FILE}

while true
do
    sleep 300
    echo "port-forward still sleeping"
done


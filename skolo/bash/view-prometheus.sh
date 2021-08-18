#!/bin/bash
set -e

# Port-forwards the Prometheus server on the given rack to the desktop and
# launches a browser.

if [ $# -ne 1 ]; then
    echo "$0 <rackN>"
    exit 1
fi

# Capture the command-line arguement.
JUMPHOST=$1

# Pick a random port to avoid conflicts.
PORT=$(shuf -i 10000-11000 -n 1)

# Set up an exit trap to shut down the ssh port forward.
function finish {
    ssh -S /tmp/skolo-prometheus-tunnel-$PORT -O exit $JUMPHOST
}
trap finish EXIT

# Start ssh port forward.
ssh -N -L $PORT:localhost:8000 $JUMPHOST -S /tmp/skolo-prometheus-tunnel-$PORT &

# Wait for ssh port forward to come up.
until nc -z localhost $PORT
do
    sleep 1
    echo "Waiting for port-forward to come up."
done

# Launch the browser to load Prometheus.
google-chrome http://localhost:$PORT

# Wait to exit.
read -r -p "Press enter when you are done." key
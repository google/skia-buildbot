#!/bin/bash

# Starts a port-forward of the CockroachDB connection.

set -e

printf "After this command launches run the following in another terminal:\n\n\tcockroach sql --insecure --host=127.0.0.1:25000\n\n"

set -x

../../kube/attach.sh skia-infra-public kubectl port-forward -n perf perf-cockroachdb-0 25000:26257

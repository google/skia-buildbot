#!/bin/bash

# Run the cockroachdb client in the cluster.

kubectl run machineserver-cockroachdb-cli -it \
--image=cockroachdb/cockroach:v19.2.5 \
--rm \
--restart=Never \
-- sql \
--insecure \
--host=machineserver-cockroachdb-public
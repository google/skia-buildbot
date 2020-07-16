#!/bin/bash
kubectl run androidx-cockroachdb -it \
--image=cockroachdb/cockroach:v19.2.5 \
--rm \
--restart=Never \
-- sql \
--insecure \
--host=perf-cockroachdb-public
#!/bin/bash
kubectl run cockroachdb -it \
--image=cockroachdb/cockroach:v19.2.5 \
--rm \
--restart=Never \
-- sql \
--insecure \
--host=perf-flutter-flutter-cockroachdb-public
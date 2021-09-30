#!/bin/bash
set -e
set -x

# Runs the batch-delete.sql, which deletes params in small batches. Edit
# batch-delete.sql directly to control which params get deleted.
#
# This script presumes that you have already run a port-forward to the cdb
# instance:
#
#     kubectl port-forward perf-cockroachdb-0 26257

while :
do
	cockroach sql --insecure --host=localhost  < ./batch-delete.sql
	sleep 5
done
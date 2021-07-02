#!/bin/bash

set -e

FILES="./configs/*.json"
for f in $FILES
do
  echo "Validating $f"
  perf-tool config validate --config_filename=$f
done
echo "Success"
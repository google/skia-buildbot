#!/bin/bash

rm $HOME/regression-traces.json
rm $HOME/no-regression-traces.json

# SKP update with big change occurs at 51050.
perf-tool traces export --config_filename=./configs/cdb-nano.json --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --begin=51049 --end=51050 --filename=$HOME/regression-traces.json --query='sub_result=min_ms'

# No alerts triggered on the previous two commits.

perf-tool traces export --config_filename=./configs/cdb-nano.json --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --begin=51043 --end=51044 --filename=$HOME/no-regression-traces.json --query='sub_result=min_ms'

echo "Regression in SKPs"
perf-try $HOME/regression-traces.json

echo "No Regressions"
perf-try $HOME/no-regression-traces.json
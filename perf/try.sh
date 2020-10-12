#!/bin/bash

#rm $HOME/regression-traces.json
#rm $HOME/no-regression-traces.json

# SKP update with big change occurs at 51050.
#perf-tool traces export --config_filename=./configs/cdb-nano.json --connection_string=postgresql://root@localhost:26257/#skia?sslmode=disable --begin=51049 --end=51050 --filename=$HOME/regression-traces.json --query='sub_result=min_ms'

# Subtle change in skp at 50867
#perf-tool traces export --config_filename=./configs/cdb-nano.json --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --begin=50866 --end=50867 --filename=$HOME/regression-traces.json --query='sub_result=min_ms&source_type=skp'

# Medium regression at 50547, pull out 20 commits before.
# https://perf.skia.org/e/?begin=1599082066&end=1599154478&keys=Xa31fdeef67781ea062c2f004217e08f0&num_commits=50&request_type=1&xbaroffset=50547
#perf-tool traces export --config_filename=./configs/cdb-nano.json --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --begin=50540 --end=50561 --filename=$HOME/regression-traces.json --query='sub_result=min_ms&source_type=skp'

# No alerts triggered on the previous two commits.
#perf-tool traces export --config_filename=./configs/cdb-nano.json --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --begin=51043 --end=51044 --filename=$HOME/no-regression-traces.json --query='sub_result=min_ms&source_type=skp'

echo "Regression in SKPs"
perf-try $HOME/regression-traces.json

#echo "No Regressions"
#perf-try $HOME/no-regression-traces.json
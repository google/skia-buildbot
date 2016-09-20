#!/bin/bash
#
# Downloads the last 1000 perf runs isolated outputs to ./downloads/.

swarming.py query \
  -S chromium-swarm.appspot.com \
  --limit=1000 \
  'tasks/list?tags=pool:Skia&tags=name:perf_skia&state=COMPLETED' \
  | grep -A 1 outputs_ref \
  | grep isolated \
  | cut -d\" -f4 \
  | xargs -I '{}' isolateserver.py download \
    --isolate-server=https://isolateserver.appspot.com \
    --isolated='{}' \
    --target=./downloads/'{}'

#!/bin/bash
#
# Update the code running on skiaperf.com
#
set -x

source ./vm_config.sh

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER skia-testing-b \
  "cd ~/buildbot/perf/setup;" \
  "git pull;" \
  "./perf_setup.sh"

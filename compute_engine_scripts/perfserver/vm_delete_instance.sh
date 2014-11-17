#!/bin/bash
#
# Deletes the compute instance for skiaperf.com
#
set -x

source ./vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance $INSTANCE_NAME

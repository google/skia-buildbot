#!/bin/bash
#
# Delete the compute instance for skiaperf.com
#

source ./vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance $INSTANCE_NAME

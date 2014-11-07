#!/bin/bash
#
# Delete the compute instance for skfiddle.com
#
set -x

source ./vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance $INSTANCE_NAME

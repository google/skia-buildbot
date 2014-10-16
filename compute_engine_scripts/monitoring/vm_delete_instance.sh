#!/bin/bash
#
# Delete the compute instance for skiamonitor.com.
#

source ../buildbots/vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance --zone=$ZONE skia-monitoring
gcutil --project=$PROJECT_ID deletedisk --zone=$ZONE  skia-monitoring-data

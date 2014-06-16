#!/bin/bash
#
# Delete the compute instance for skiamonitor.com.
#

source ../buildbots/vm_config.sh

gcutil --project=$PROJECT_ID deleteinstance skia-monitoring-$ZONE_TAG --zone=$ZONE

#/bin/bash

# Creates the pager duty secret.

set -e -x

source ../kube/config.sh
source ../bash/ramdisk.sh

cd /tmp/ramdisk

echo "Replace the entire file contents with just the PagerDuty secret." > /tmp/ramdisk/pagerduty.txt
${EDITOR} /tmp/ramdisk/pagerduty.txt

kubectl create secret generic pager-duty --from-file=pagerduty.txt
cd -

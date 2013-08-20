#!/bin/bash
#
# This script copies the buildbot history from the master in an old zone to the
# master in the new zone.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

OLD_MASTER_HOSTNAME="${VM_NAME_BASE}-${VM_MASTER_NAME}-${OLD_ZONE_TAG}"
NEW_MASTER_HOSTNAME="${VM_NAME_BASE}-${VM_MASTER_NAME}-${ZONE_TAG}"

BUILDBOT_HISTORY_FILES="http.log* Build-* Canary-* Housekeeper-* Perf-* Test-* state.sqlite twistd.log*"

for BUILDBOT_HISTORY_FILE in $BUILDBOT_HISTORY_FILES; do

  echo "===== Archiving $BUILDBOT_HISTORY_FILE on the old master ($OLD_MASTER_HOSTNAME) ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $OLD_MASTER_HOSTNAME \
    "if [[ -d /home/default/skia-master/buildbot/master ]]; then cd /home/default/skia-master/buildbot/master; else cd /home/default/skia-repo/buildbot/master; fi && " \
    "tar -cz --ignore-failed-read -f /tmp/buildbot-history.tgz $BUILDBOT_HISTORY_FILE && " \
    "scp -o UserKnownHostsFile=/dev/null -o CheckHostIP=no -o StrictHostKeyChecking=no /tmp/buildbot-history.tgz ${PROJECT_USER}@${NEW_MASTER_HOSTNAME}:/home/default/skia-repo/buildbot/master/ && " \
    "rm -rf /tmp/buildbot-history.tgz"

  if [[ $? != "0" ]]; then
    echo "Archiving $BUILDBOT_HISTORY_FILE on the old master failed!"
    exit 1
  fi

  echo "===== Unpacking $BUILDBOT_HISTORY_FILE on the new master ($NEW_MASTER_HOSTNAME) ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $NEW_MASTER_HOSTNAME \
    "cd /home/default/skia-repo/buildbot/master && " \
    "tar --overwrite -xzf buildbot-history.tgz && " \
    "rm buildbot-history.tgz"

  if [[ $? != "0" ]]; then
    echo "Unpacking $BUILDBOT_HISTORY_FILE on the new master failed!"
    exit 1
  fi

done

echo
echo "===== Completed transfering history from $OLD_MASTER_HOSTNAME to $NEW_MASTER_HOSTNAME ====="
echo

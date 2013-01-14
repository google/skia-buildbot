#!/bin/bash
#
# Logs into the specified Skia compute engine instance, parses out the
# persistent disk usage and compares it against the threshold.
#
# The SKIA_COMPUTE_ENGINE_HOSTNAME environment variable is the hostname of the
# compute engine instance we want to check. The PERSISTENT_DISK_NAME is the
# mounted path of the disk we want to check.
#
# Sample Usage:
#  SKIA_COMPUTE_ENGINE_HOSTNAME=skia-master-a.c.skia-buildbots.google.com.internal \
#  PERSISTENT_DISK_NAME=/home/default/skia-master \
#  bash check_compute_engine_disk_usage.sh
#
# Can also optionally specify the environment variable THRESHOLD (default 90).
#

THRESHOLD=${THRESHOLD:-90}

# Check to see if the script can log into the compute engine instance.
ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no -o \
StrictHostKeyChecking=no -p 22 $SKIA_COMPUTE_ENGINE_HOSTNAME 'df -h'
ret_code=`echo $?`
if [ "$ret_code" -ne 0 ]; then
  echo -e "There was an error logging into the compute engine instance! Return code: $ret_code"
  exit $ret_code
fi

# Log into the compute engine instance and parse the percentage used of the
# persistent disk.
complete_output=`ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no -o \
StrictHostKeyChecking=no -p 22 $SKIA_COMPUTE_ENGINE_HOSTNAME 'df -h' | \
grep $PERSISTENT_DISK_NAME`; IFS=' ' v=($complete_output); \
percent_used=${v[4]/\%/}

if [ "$percent_used" -lt "$THRESHOLD" ]; then
  echo -e "\nThe percentage used ($percent_used%) is below the threshold ($THRESHOLD%).\n"
  exit 0
else
  echo -e "\nThe percentage used ($percent_used%) is at or beyond the threshold ($THRESHOLD%).\n"
  echo -e "Please make room on the compute engine instance by deleting unneeded files.\n"
  exit 1
fi


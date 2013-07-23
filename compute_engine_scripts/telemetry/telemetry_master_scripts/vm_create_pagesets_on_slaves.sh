#
# Starts the telemetry_slave_scripts/vm_create_pagesets.sh script on all
# slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ $# -ne 3 ]; then
  echo
  echo "Usage: `basename $0` rmistry@google.com 1001 All"
  echo
  echo "The first argument is the email address of the requester."
  echo "The second argument is the key of the appengine admin task."
  echo "The third argument is the type of pagesets to create from the 1M list" \
       "Eg: All, Filtered, 100k, 10k, Deeplinks."
  echo
  exit 1
fi

REQUESTER_EMAIL=$1
APPENGINE_KEY=$2
PAGESETS_TYPE=$3

source vm_utils.sh
source ../vm_config.sh

# Update buildbot.
gclient sync

# If PAGESETS_TYPE is 100k or 10k then adjust NUM_WEBPAGES.
if [ "$PAGESETS_TYPE" == "100k" ]; then
  NUM_WEBPAGES=100000
elif [ "$PAGESETS_TYPE" == "10k" ]; then
  NUM_WEBPAGES=10000
fi

NUM_WEBPAGES_PER_SLAVE=$(($NUM_WEBPAGES/$NUM_SLAVES))

START=1
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  END=$(expr $START + $NUM_WEBPAGES_PER_SLAVE - 1)
  CMD="bash vm_create_pagesets.sh $SLAVE_NUM $START $PAGESETS_TYPE"
  START=$(expr $END + 1)
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; svn update; $CMD > /tmp/create_pagesets_output.txt 2>&1"
done

# Check to see if the slaves are done creating page_sets.
SLAVES_STILL_PROCESSING=true
while $SLAVES_STILL_PROCESSING ; do
  SLAVES_STILL_PROCESSING=false
  for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
    RET=$( is_slave_currently_executing $SLAVE_NUM $CREATING_PAGESETS_ACTIVITY )
    if $RET; then
      echo "skia-telemetry-worker$SLAVE_NUM is still running $CREATING_PAGESETS_ACTIVITY"
      echo "Sleeping for a minute and then retrying"
      SLAVES_STILL_PROCESSING=true
      sleep 60
      break
    else
      echo "skia-telemetry-worker$SLAVE_NUM is done processing."
    fi
  done
done

BOUNDARY=`date +%s|md5sum`
BOUNDARY=${BOUNDARY:0:32}
sendmail $REQUESTER_EMAIL <<EOF
subject:Your Recreate Pagesets task has completed!
to:$REQUESTER_EMAIL
from:skia.buildbot@gmail.com
Content-Type: multipart/mixed; boundary=\"$BOUNDARY\";

This is a MIME-encapsulated message

--$BOUNDARY
Content-Type: text/html

<html>
  <head/>
  <body>
  You can schedule more runs <a href='https://skia-tree-status.appspot.com/skia-telemetry/lua_script'>here</a>.<br/><br/>
  Thanks!
  </body>
</html>

--$BOUNDARY--

EOF

# Mark this task as completed on AppEngine.
PASSWORD=`cat /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt`
wget --post-data "key=$APPENGINE_KEY&password=$PASSWORD" "https://skia-tree-status.appspot.com/skia-telemetry/update_admin_task" -O /dev/null

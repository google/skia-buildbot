#!/bin/bash
#
# Create and setup the Skia RecreateSKPs GCE instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_RECREATESKPS_BOT[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

# Create the Skia recreate SKPs instance.
# TODO(rmistry): Update the below command to use the new skiabuildbot image
# after it is captured.
$GCOMPUTE_CMD addinstance ${VM_RECREATESKPS_BOT_NAME} \
  --zone=$ZONE \
  --external_ip_address=${VM_RECREATESKPS_BOT_IP_ADDRESS} \
  --service_account=default \
  --service_account_scopes="$SCOPES" \
  --network=skia \
  --image=skiatelemetry-6-0-ubuntu1310 \
  --machine_type=lmt-n1-standard-8-d \
  --auto_delete_boot_disk

if [ $? -ne 0 ]
then
  echo
  echo "===== There was an error creating the instance. ====="
  echo
  exit 1
fi

echo "===== Wait 3 mins for the instance to come up. ====="
sleep 180

FAILED=""

SKIA_REPO_DIR="/home/default/storage/skia-repo"

echo "Checkout Skia Buildbot code"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_RECREATESKPS_BOT_NAME \
  "mkdir $SKIA_REPO_DIR && " \
  "cd $SKIA_REPO_DIR && " \
  "~/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
  "~/depot_tools/gclient sync;" \
  || FAILED="$FAILED CheckoutSkiaBuildbot"
echo

echo "Checkout Skia Trunk code"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_RECREATESKPS_BOT_NAME \
  "cd $SKIA_REPO_DIR && " \
  "sed -i '$ d' .gclient && sed -i '$ d' .gclient && " \
  "echo \"\"\"
  { 'name'        : 'skia',
    'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
]
\"\"\" >> .gclient && ~/depot_tools/gclient sync;" \
  || FAILED="$FAILED CheckoutSkiaTrunk"
echo

if [[ $FAILED ]]; then
  echo
  echo "FAILURES: $FAILED"
  echo "Please manually fix these errors."
  echo
fi

echo
echo "===== Copying over required files. ====="
  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_RECREATESKPS_BOT[@]}; do
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_RECREATESKPS_BOT_NAME \
      $REQUIRED_FILE /home/default/
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_RECREATESKPS_BOT_NAME \
      $REQUIRED_FILE /home/default/storage/
  done
echo

cat <<INP
If you did not see a table which looked like
+---------------------+-------------------------------------------
| name                | operation-1327681189228-4b784dda81d58-b99dd05c |
| status              | DONE                                           |
| target              | ${VM_RECREATESKPS_BOT_NAME}
 ...
| operationType       | insert                                         |

Then the vm name may be running already. You will have to delete it to
recreate it with different atttributes or move it to a different zone.

Wait until the status is RUNNING


SSH into the instance with:
  gcutil --project=google.com:chromecompute ssh --ssh_user=default ${VM_RECREATESKPS_BOT_NAME}
and run the following commands:
  * cd $SKIA_REPO_DIR
  * nohup python buildbot/scripts/launch_slaves.py &

INP

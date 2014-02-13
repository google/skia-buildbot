#!/bin/bash
#
# Create and setup the Skia Commit Queue GCE instance.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_CQ[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

# Create the Skia CQ instance.
$GCOMPUTE_CMD addinstance ${VM_CQ_NAME} \
  --zone=$ZONE \
  --external_ip_address=${VM_CQ_IP_ADDRESS} \
  --service_account=default \
  --service_account_scopes="$SCOPES" \
  --network=skia \
  --image=skiatelemetry-1-0-v20130524 \
  --machine_type=n1-standard-8-d \
  --nopersistent_boot_disk \
  --service_version=v1beta16

if [ $? -ne 0 ]
then
  echo
  echo "===== There was an error creating the instance. ====="
  echo
  exit 1
fi

FAILED=""

COMMIT_QUEUE_DIR="/home/default/storage/skia-commit-queue"

echo "===== Checkout commit-queue-internal. ====="
  echo "Use the password from https://chromium-access.appspot.com/"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_CQ_NAME \
    "mkdir $COMMIT_QUEUE_DIR && " \
    "cd $COMMIT_QUEUE_DIR && " \
    "/home/default/depot_tools/gclient config svn://svn.chromium.org/chrome-internal/trunk/tools/commit-queue --name commit-queue && " \
    "svn ls svn://svn.chromium.org/chrome-internal --username rmistry@google.com && " \
    "/home/default/depot_tools/gclient sync" \
    || FAILED="$FAILED CheckoutCommitQueueInternal"
echo

echo "===== Install necessary packages. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_CQ_NAME \
    "sudo apt-get -y install python-mechanize " \
    || FAILED="$FAILED InstallPackages"
echo

if [[ $FAILED ]]; then
  echo
  echo "FAILURES: $FAILED"
  echo "Please manually fix these errors."
  echo
fi

echo
echo "===== Copying over required files. ====="
  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_CQ[@]}; do
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_CQ_NAME \
      $REQUIRED_FILE $COMMIT_QUEUE_DIR/commit-queue/workdir/
  done
echo

cat <<INP
If you did not see a table which looked like
+---------------------+-------------------------------------------
| name                | operation-1327681189228-4b784dda81d58-b99dd05c |
| status              | DONE                                           |
| target              | ${VM_CQ_NAME}
 ...
| operationType       | insert                                         |

Then the vm name may be running already. You will have to delete it to
recreate it with different atttributes or move it to a different zone.

Check ./vm_status.sh to wait until the status is RUNNING


SSH into the CQ with:
  gcutil --project=google.com:chromecompute ssh --ssh_user=default ${VM_CQ_NAME}
and start the commit queue for Skia using the following commands:
  * Create ~/.netrc using skia-commit-bot's password from valentine for googlesource.
  * cd ${COMMIT_QUEUE_DIR}/commit-queue
  * Apply the patch from https://chromereviews.googleplex.com/24067015/ (if it is not already submitted).
  * git config --global user.email "skia-commit-bot@chromium.org"
  * git config --global user.name "Skia Commit Bot"
  * Start the CQ with: python commit_queue.py --project=skiabuildbot --no-dry-run --user=skia-commit-bot@chromium.org

INP

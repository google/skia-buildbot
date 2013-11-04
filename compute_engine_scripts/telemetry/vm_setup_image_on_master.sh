#!/bin/bash
#
# Setup the telemetry instance image.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

VM_COMPLETE_NAME="${VM_NAME_BASE}-${VM_MASTER_NAME}"

REQUIRED_FILES_FOR_IMAGE=(~/.boto)

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_IMAGE[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done


echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

FAILED=""

echo "Install required packages."
# TODO(rmistry): No parallel package for ubuntu, it is required for pdfviewer.
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo apt-get update; " \
  "sudo apt-get -y install git make subversion postfix python-dev xvfb python-twisted-core " \
  "vim gyp g++ gdb unzip linux-tools libgif-dev;" \
  || FAILED="$FAILED InstallPackages"
echo

echo "Update gsutil."
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo gsutil update;" \
  || FAILED="$FAILED Update gsutil"
echo

echo "Setup .bashrc."
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "echo 'PATH=\"/home/default/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH\"' >> ~/.bashrc && " \
  "echo 'alias ll=\"ls -l\"' >> ~/.bashrc && " \
  "echo 'alias m=\"cd /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts\"' >> ~/.bashrc && " \
  "echo 'alias s=\"cd /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts\"' >> ~/.bashrc;" \
  || FAILED="$FAILED SetupBashrc"
echo

echo "Remove boto.cfg"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo rm -rf /etc/boto.cfg" \
  || FAILED="$FAILED RemoveBotoCfg"
echo

echo "Setup automount script"
$GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
  image-files/automount-sdb /tmp/
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo cp /tmp/automount-sdb /etc/init.d/ && " \
  "sudo chmod 755 /etc/init.d/automount-sdb && " \
  "sudo update-rc.d automount-sdb defaults &&" \
  "sudo /etc/init.d/automount-sdb start" \
  || FAILED="$FAILED SetupAutomountScript"
echo

echo "Checkout depot_tools"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "svn checkout http://src.chromium.org/svn/trunk/tools/depot_tools;" \
  || FAILED="$FAILED CheckoutDepotTools"
echo

echo "Checkout Skia Buildbot code"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "mkdir ~/skia-repo/ && " \
  "cd ~/skia-repo/ && " \
  "~/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
  "~/depot_tools/gclient sync;" \
  || FAILED="$FAILED CheckoutSkiaBuildbot"
echo

echo "Checkout Skia Trunk code"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "cd ~/skia-repo/ && " \
  "sed -i '$ d' .gclient && sed -i '$ d' .gclient && " \
  "echo \"\"\"
  { 'name'        : 'trunk',
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

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_IMAGE[@]}; do
  $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
    $REQUIRED_FILE /home/default/
done

if [[ $FAILED ]]; then
  echo
  echo "FAILURES: $FAILED"
  echo "Please manually fix these errors."
  echo
fi

echo
echo "SSH into the master with:"
echo "gcutil --project=google.com:chromecompute ssh --ssh_user=default skia-telemetry-master"
echo "* Follow the instructions in https://developers.google.com/compute/docs/networking#mailserver using skia.buildbots@gmail.com"
echo "* Run 'gclient sync' in /home/default/skia-repo/buildbot and enter the correct AppEngine password in /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts/appengine_password.txt"
echo "* Run Chromium's install-build-deps.sh"
echo "* Run 'git config --global user.name' and 'git config --global user.email'"
echo

echo
echo "You can take an image by running the following commands:"
echo "sudo python /usr/share/imagebundle/image_bundle.py -r / -o /tmp/ --log_file=/tmp/image.log"
echo "Copy the image to Google Storage."
echo "* gsutil config"
echo "* gsutil cp /tmp/<your-image>.image.tar.gz gs://skia-images-1/"
echo "gcutil --project=google.com:chromecompute addimage skiatelemetry-2-0-v20131101 gs://skia-images-1/<your-image>.image.tar.gz --preferred_kernel=projects/google/global/kernels/gce-v20130325"
echo


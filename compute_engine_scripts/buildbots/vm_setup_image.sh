#!/bin/bash
#
# Setups the instance image on Skia GCE instance.
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
  "sudo apt-get -y install subversion git make postfix python-dev libfreetype6-dev xvfb python-twisted-core libpng-dev zlib1g-dev fontconfig libfontconfig-dev libglu-dev " \
  "vim gyp g++ gdb unzip linux-tools libgif-dev python-imaging libosmesa-dev linux-tools-3.11.0-17-generic && " \
  "sudo apt-get install gcc python-dev python-setuptools && sudo easy_install -U pip && sudo pip install setuptools --no-use-wheel --upgrade && sudo pip install -U crcmod" \
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
  "git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git" \
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
echo "You can take an image by running the following commands:"
echo "sudo gcimagebundle -d /dev/sda -o /tmp/ --log_file=/tmp/image.log"
echo "Copy the image to Google Storage."
echo "* gsutil config"
echo "* gsutil cp /tmp/<your-image>.image.tar.gz gs://skia-images-1/"
echo "gcutil --project=google.com:skia-buildbots addimage skiatelemetry-2-0-v20131101 gs://skia-images-1/<your-image>.image.tar.gz --preferred_kernel=projects/google/global/kernels/gce-v20130325"
echo


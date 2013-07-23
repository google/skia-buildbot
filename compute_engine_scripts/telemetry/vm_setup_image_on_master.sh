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
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo dpkg --add-architecture i386; " \
  "sudo apt-get update; " \
  "sudo apt-get -y install git make subversion postfix python-dev xvfb python-twisted-core " \
  "vim gyp g++ pkg-config libgtk2.0-dev libglib2.0-dev libnss3-dev libgconf2-dev " \
  "libpci-dev libgcrypt11-dev libgnome-keyring-dev libudev-dev libpulse-dev " \
  "libcups2-dev libelf-dev gperf libbison-dev ia32-libs libxtst-dev libasound2-dev libxss-dev " \
  "xfonts-100dpi xfonts-75dpi xfonts-scalable xfonts-cyrillic xserver-xorg-core " \
  "ttf-.*-fonts fonts-nanum fonts-tlwg-* fonts-kacst.* fonts-thai-tlwg libgl1-mesa-dev " \
  "libgif-dev libglu-dev gdb unzip;" \
  || FAILED="$FAILED InstallPackages"
echo

echo "Setup .bashrc."
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "echo 'PATH=\"/home/default/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH\"' >> ~/.bashrc && " \
  "echo 'alias ll=\"ls -l\"' >> ~/.bashrc;" \
  || FAILED="$FAILED SetupBashrc"
echo

echo "Remove boto.cfg"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo rm -rf /etc/boto.cfg" \
  || FAILED="$FAILED RemoveBotoCfg"
echo

echo "Setup automount script"
$GCOMPUTE_CMD push --ssh_user=root $VM_COMPLETE_NAME \
  image-files/automount-sdb /etc/init.d/
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "sudo chmod 755 /etc/init.d/automount-sdb && " \
  "sudo update-rc.d automount-sdb defaults" \
  || FAILED="$FAILED CheckoutDepotTools"
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
  "~/depot_tools/gclient config https://skia.googlecode.com/svn/buildbot && " \
  "~/depot_tools/gclient sync;" \
  || FAILED="$FAILED CheckoutSkiaBuildbot"
echo

echo "Checkout Skia Trunk code"
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
  "cd ~/skia-repo/ && " \
  "sed -i '$ d' .gclient && " \
  "echo \"\"\"
  { 'name'        : 'trunk',
    'url'         : 'https://skia.googlecode.com/svn/trunk',
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

echo "Rebooting the machine..."
$GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME "sudo reboot;"
echo

if [[ $FAILED ]]; then
  echo
  echo "FAILURES: $FAILED"
  echo "Please manually fix these errors."
  echo
fi

echo
echo "You can take an image by running the following commands:"
echo "sudo python /usr/share/imagebundle/image_bundle.py -r / -o /tmp/ --log_file=/tmp/abc.log"
echo "Copy the image to Google Storage."
echo "gcutil --project=<project-id> addimage <image-name> <image-uri> --preferred_kernel=projects/google/global/kernels/<kernel-name>"
echo


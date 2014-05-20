#!/bin/bash
#
# Setup a cluster telemetry machine.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

if [ -n "$1" ]; then
  CT_MACHINE=$1
else
  echo
  echo "Usage: bash `basename $0` build1-b5"
  echo
  echo "The first argument is the hostname or IP address of the Cluster" \
       "Telemetry machine we want to setup."
  echo
  exit 1
fi

REQUIRED_FILES_FOR_CT_MACHINE=(~/.boto \
                               ~/.bashrc \
                               ~/.inputrc \
                               /b/google-cloud-sdk)

echo """
Before proceeding with this script please ensure that:
* This machine can access $CT_MACHINE with passwordless ssh.
* chrome-bot has passwordless sudo access on $CT_MACHINE.
"""
read -p "Press [Enter] key to continue..."

echo """

================================================
Starting setup of ${CT_MACHINE}.....
================================================

"""

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_CT_MACHINE[@]}; do
  if [ ! -e $REQUIRED_FILE ];
  then
    echo "This machine is missing $REQUIRED_FILE!"
    exit 1
  else
    scp -r -o UserKnownHostsFile=/dev/null -o CheckHostIP=no -o StrictHostKeyChecking=no ${REQUIRED_FILE} ${CT_MACHINE}:${REQUIRED_FILE}
  fi
done


CMD="""
# Install required packages.
sudo apt-get update;
sudo apt-get -y install linux-tools python-django libgif-dev lua5.2 && sudo easy_install -U pip && sudo pip install setuptools --no-use-wheel --upgrade && sudo pip install -U crcmod;

# Checkout Skia's buildbot and trunk repositories.
mkdir /b/storage/;
mkdir /b/skia-repo/;
cd /b/skia-repo/;
/b/depot_tools/gclient config https://skia.googlesource.com/buildbot.git;
/b/depot_tools/gclient sync;
sed -i '$ d' .gclient && sed -i '$ d' .gclient;
echo \"\"\"
  { 'name'        : 'trunk',
    'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
]
\"\"\" >> .gclient;
/b/depot_tools/gclient sync;

# Copy required patches from Google Storage.
gsutil cp gs://chromium-skia-gm/telemetry/patches/desktop_browser_backend.py /b/skia-repo/buildbot/third_party/chromium_trunk/src/tools/telemetry/telemetry/core/backends/chrome/desktop_browser_backend.py;

# Create symlink from /b to /home/default for the old page_set paths.
sudo ln -s /b /home/default;

# Setup a headless xserver.
sudo apt-get -y install lightdm-gtk-greeter mesa-utils;
sudo gsutil cp gs://chromium-skia-gm/telemetry/patches/edid.bin /etc/X11/;
sudo gsutil cp gs://chromium-skia-gm/telemetry/patches/xorg.conf /etc/X11/;
sudo service lightdm start;
echo 'sudo cp /var/run/lightdm/root/:0 ~/.Xauthority' >> ~/.bashrc;
echo 'sudo chown chrome-bot:chrome-bot ~/.Xauthority' >> ~/.bashrc;
sudo reboot;
"""
ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no -o StrictHostKeyChecking=no \
  -A -q -p 22 $CT_MACHINE -- "$CMD"


echo
echo "The setup script has completed!"
echo
echo "Remaining steps:"
echo "* If you are setting up the master then enter the correct AppEngine password in /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts/appengine_password.txt"
echo

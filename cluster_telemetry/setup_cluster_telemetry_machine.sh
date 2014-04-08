#!/bin/bash
#
# Setup a cluster telemetry machine.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source config.sh

VM_COMPLETE_NAME=`hostname`

echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

FAILED=""

echo "==Install required packages=="
sudo apt-get update;
sudo apt-get install linux-tools python-django libgif-dev && sudo easy_install -U pip && sudo pip install setuptools --no-use-wheel --upgrade && sudo pip install -U crcmod \
|| FAILED="$FAILED InstallPackages"
echo

echo "Checkout Skia Buildbot code"
mkdir /b/storage/;
mkdir /b/skia-repo/;
cd /b/skia-repo/ && \
gclient config https://skia.googlesource.com/buildbot.git && \
gclient sync \
|| FAILED="$FAILED CheckoutSkiaBuildbot"
echo

echo "Checkout Skia Trunk code"
cd /b/skia-repo/ && \
sed -i '$ d' .gclient && sed -i '$ d' .gclient && \
echo """
  { 'name'        : 'trunk',
    'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
]
""" >> .gclient && gclient sync \
|| FAILED="$FAILED CheckoutSkiaTrunk"
echo

if [[ $FAILED ]]; then
  echo
  echo "FAILURES: $FAILED"
  echo "Please manually fix these errors."
  echo
fi

echo
echo "* Copy .boto, .bashrc and .inputrc to the machine."
echo "* Install Google Cloud SDK: https://developers.google.com/cloud/sdk/#Quick_Start OR copy /b/google-cloud-sdk from another cluster telemetry machine."
echo "* Setup chrome-bot in sudoers."
echo "* Setup passwordless access from the master to the other slaves."
echo "* Run 'gclient sync' in /b/skia-repo/buildbot and enter the correct AppEngine password in /b/skia-repo/buildbot/cluster_telemetry/telemetry_master_scripts/appengine_password.txt"
echo

#!/bin/bash
#
# Recovers slaves from VM crashes.
# Recovery commands that are not a part of the image yet should go in this
# script.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../vm_config.sh

# Update buildbot and trunk.
gclient sync

for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  # After VM crashes the slaves only contain one 'lost+found' directory in the
  # mounted scratch disk. This is our indication that the VM crashed recently.
  NUM_DIRS=`ssh -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "ls -d ~/storage/*/ | wc -l"`
  if [ "$NUM_DIRS" == "1" ]; then
    CMD="""
echo Deleting SVN checkouts and creating Git checkouts;
cd skia-repo;
rm -rf buildbot trunk;
sed -i 's/https\:\/\/skia\.googlecode\.com\/svn\/buildbot/https\:\/\/skia\.googlesource\.com\/buildbot\.git/g' .gclient;
sed -i 's/https\:\/\/skia\.googlecode\.com\/svn\/trunk/https\:\/\/skia\.googlesource\.com\/skia\.git/g' .gclient;
/home/default/depot_tools/gclient sync;
echo =====Installing packages missing from the image=====;
sudo apt-get update;
sudo apt-get -y install libgif-dev libgl1-mesa-dev libglu-dev gdb unzip linux-tools parallel;
echo =====Updating gsutil on the slave=====;
sudo gsutil update -n;
echo =====Adding the testing repository to sources.list and installing libc6=====
sudo sh -c \"echo 'deb ftp://ftp.fr.debian.org/debian/ testing main contrib  non-free' >> /etc/apt/sources.list\";
sudo apt-get update;
sudo DEBIAN_FRONTEND=noninteractive apt-get -y -t testing install libc6-dev;
"""
    ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
      -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
      -A -q -p 22 default@108.170.222.$SLAVE_NUM -- "$CMD"
  fi
done


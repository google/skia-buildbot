#!/bin/bash
#
# Setup all the software running on skfiddle.com
#
set -x

source ./vm_config.sh

# Basically SSH in, clone this repo and jump to a shell script in the repo.

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
  "sudo apt-get -y update;" \
  "sudo apt-get -y upgrade;" \
  "sudo apt-get -y install git;" \
  "git clone https://skia.googlesource.com/buildbot;" \
  "cd ~/buildbot/webtry/setup;" \
  "bash ./webtry_setup.sh"

echo "Make sure to 'set daemon 2' in /etc/monit/monitrc"

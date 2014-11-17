#!/bin/bash
#
# Setups all the software running on skiamonitor.com.
#
set -x

source vm_config.sh

# Basically SSH in, clone this repo and jump to a shell script in the repo.

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER $VM_NAME_BASE-monitoring \
  "sudo mkdir -p /mnt/pd0;" \
  "sudo /usr/share/google/safe_format_and_mount -m \"mkfs.ext4 -F\" /dev/disk/by-id/google-skia-monitoring-data /mnt/pd0; " \
  "sudo chmod 777 /mnt/pd0;" \
  "sudo apt-get -y update;" \
  "sudo apt-get -y upgrade;" \
  "sudo apt-get -y install git mercurial;" \
  "git clone https://skia.googlesource.com/buildbot;" \
  "cd ~/buildbot;" \
  "cd compute_engine_scripts/monitoring;" \
  "bash ./setup.sh"

echo "Make sure to 'set daemon 2' in /etc/monit/monitrc"
echo "Make sure to log in InfluxDB and create the 'graphite' and 'grafana' databases. Password set according to valentine."

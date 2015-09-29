#!/bin/bash
#
# Script to set up a base image with just collectd, and pulld.
#
# This script is used on a temporary GCE instance. Just run it on a fresh
# Ubuntu 15.04 image and then capture a snapshot of the disk. Any image
# started with this snapshot as its image should be immediately setup to
# install applications via Skia Push.
#
# For more details see ../../push/DESIGN.md.
sudo apt-get update
sudo apt-get --assume-yes install git
# Running "sudo apt-get --assume-yes upgrade" may upgrade the package
# gce-startup-scripts, which would cause systemd to restart gce-startup-scripts,
# which would kill this script because it is a child process of
# gce-startup-scripts.
#
# IMPORTANT: We are using a public Ubuntu image which has automatic updates
# enabled by default. Thus we are not running any commands to update packages.

sudo apt-get --assume-yes -o Dpkg::Options::="--force-confold" install collectd
gsutil cp gs://skia-push/debs/pulld/pulld:stephana@stephana2.cnc.corp.google.com:2015-09-16T14:41:21Z:a2a216c62a5b280fc444a2766b4ff3d61918915f.deb pulld.deb
sudo dpkg -i pulld.deb
sudo apt-get --assume-yes install --fix-broken

# Setup collectd.
sudo cat <<EOF > collectd.conf
FQDNLookup false
Interval 10

LoadPlugin "logfile"
<Plugin "logfile">
  LogLevel "info"
  File "/var/log/collectd.log"
  Timestamp true
</Plugin>

LoadPlugin syslog

<Plugin syslog>
        LogLevel info
</Plugin>

LoadPlugin battery
LoadPlugin cpu
LoadPlugin df
LoadPlugin disk
LoadPlugin entropy
LoadPlugin interface
LoadPlugin irq
LoadPlugin load
LoadPlugin memory
LoadPlugin processes
LoadPlugin swap
LoadPlugin users
LoadPlugin write_graphite

<Plugin write_graphite>
        <Carbon>
                Host "skia-monitoring"
                Port "2003"
                Prefix "collectd."
                StoreRates false
                AlwaysAppendDS false
                EscapeCharacter "_"
                Protocol "tcp"
        </Carbon>
</Plugin>
EOF
sudo install -D --verbose --backup=none --group=root --owner=root --mode=600 collectd.conf /etc/collectd/collectd.conf
sudo /etc/init.d/collectd restart

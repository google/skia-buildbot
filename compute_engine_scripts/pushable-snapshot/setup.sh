#!/bin/bash
#
# Script to set up a base image with just monit, collectd, and pull.
#
# This script is used on a temporary GCE instance. Just run it on a fresh
# wheezybackports image and then capture a snapshot of the disk. Any image
# started with this snapshot as its image should be immediately setup to
# install applications via Skia Push.
#
# For more details see ../../push/DESIGN.md.
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install monit collectd
gsutil cp gs://skia-push/debs/pull/pull:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-11T14:20:31Z:e7c68f93827f1e651b45d3bc07d72de11e0eac8b.deb pull.deb
sudo dpkg -i pull.deb

# Setup monit.
cat <<EOF > monitrc
set daemon 2
set logfile /var/log/monit.log
set idfile /var/lib/monit/id
set statefile /var/lib/monit/state
set eventqueue
    basedir /var/lib/monit/events # set the base directory where events will be stored
    slots 100                     # optionally limit the queue size

set httpd port 10114
  allow admin:admin

include /etc/monit/conf.d/*
EOF
sudo install -D --verbose --backup=none --group=root --owner=root --mode=600 monitrc /etc/monit/monitrc

sudo chmod 600 /etc/monit/monitrc
sudo monit reload

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
        </Carbon>
</Plugin>
EOF
sudo install -D --verbose --backup=none --group=root --owner=root --mode=600 collectd.conf /etc/collectd/collectd.conf
sudo /etc/init.d/collectd restart

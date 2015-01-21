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

# Sane ulimits.
sudo cat <<EOF > limits.conf
*         hard    nofile      500000
*         soft    nofile      500000
root      hard    nofile      500000
root      soft    nofile      500000
EOF
sudo install -D --verbose --backup=none --group=root --owner=root --mode=644 limits.conf /etc/security/limits.conf

sudo cat <<EOF > common-session
#
# /etc/pam.d/common-session - session-related modules common to all services
#
# This file is included from other service-specific PAM config files,
# and should contain a list of modules that define tasks to be performed
# at the start and end of sessions of *any* kind (both interactive and
# non-interactive).
#
# As of pam 1.0.1-6, this file is managed by pam-auth-update by default.
# To take advantage of this, it is recommended that you configure any
# local modules either before or after the default block, and use
# pam-auth-update to manage selection of other modules.  See
# pam-auth-update(8) for details.

# here are the per-package modules (the "Primary" block)
session [default=1]     pam_permit.so
# here's the fallback if no module succeeds
session requisite     pam_deny.so
# prime the stack with a positive return value if there isn't one already;
# this avoids us returning an error just because nothing sets a success code
# since the modules above will each just jump around
session required      pam_permit.so
# and here are more per-package modules (the "Additional" block)
session required  pam_unix.so
session required  pam_limits.so
session optional  pam_ck_connector.so nox11
# end of pam-auth-update config
EOF
sudo install -D --verbose --backup=none --group=root --owner=root --mode=644 common-session /etc/pam.d/common-session

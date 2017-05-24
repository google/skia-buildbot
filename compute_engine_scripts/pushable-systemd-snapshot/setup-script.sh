#!/bin/bash
#
# Script to set up a base image with just collectd, and pulld.
#
# This script is used on a temporary GCE instance. Just run it on a fresh
# instance and then capture a snapshot of the disk. Any image
# started with this snapshot as its image should be immediately setup to
# install applications via Skia Push.
#
# For more details see ../../push/DESIGN.md.
set -x -e
export TERM=xterm
sudo apt update
sudo apt --assume-yes upgrade
sudo apt --assume-yes install git

# Move to testing.
sudo cat <<EOF > /etc/apt/sources.list
deb http://deb.debian.org/debian/ testing main
deb-src http://deb.debian.org/debian/ testing main
deb http://security.debian.org/ testing/updates main
deb-src http://security.debian.org/ testing/updates main
EOF

sudo apt update
sudo apt full-upgrade
sudo apt autoremove
sudo apt update
sudo apt full-upgrade
sudo apt autoremove
sudo apt update
sudo apt full-upgrade
sudo apt autoremove
sudo apt update
sudo apt full-upgrade
sudo apt autoremove

sudo apt --assume-yes -o Dpkg::Options::="--force-confold" install collectd
sudo gsutil cp  gs://skia-push/debs/pulld/ pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2017-03-02T16:55:37Z:38251c8ddc7f1033dd92064735aa45aedb48f527.deb pulld.deb
sudo dpkg -i pulld.deb
sudo systemctl start pulld.service

sudo apt --assume-yes install --fix-broken

# Setup collectd.
sudo cat <<EOF > collectd.conf
FQDNLookup false
Interval 60

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

sudo apt install unattended-upgrades
sudo DEBCONF_DB_OVERRIDE="File{/tmp/override.dat readonly:true}" dpkg-reconfigure  --priority=low --frontend=readline  unattended-upgrades

sudo cat <<EOF > /etc/apt/apt.conf.d/50unattended-upgrades
Unattended-Upgrade::Origins-Pattern {
      "o=Debian,a=testing";
};
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "true";
EOF

# Check the output to confirm that the config is working.
sudo unattended-upgrades -d

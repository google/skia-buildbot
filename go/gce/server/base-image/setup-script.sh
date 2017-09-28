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

echo 'Dpkg::Progress-Fancy "0";' | sudo tee /etc/apt/apt.conf.d/99progressbar

sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" update
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" upgrade
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" autoremove

# Remove unused packages.
sudo apt-get --assume-yes --purge remove dnsmasq*

# Now install the apps that we guarantee to appear.
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install git collectd unattended-upgrades
gsutil cp gs://skia-push/debs/pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2017-03-02T16:55:37Z:38251c8ddc7f1033dd92064735aa45aedb48f527.deb pulld.deb
sudo dpkg -i pulld.deb
sudo systemctl start pulld.service

sudo apt --assume-yes install --fix-broken

# Setup collectd.
cat <<EOF > collectd.conf
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

cat <<EOF | sudo tee /etc/apt/apt.conf.d/20auto-upgrades
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF

cat <<EOF | sudo tee /etc/apt/apt.conf.d/50unattended-upgrades
Unattended-Upgrade::Origins-Pattern {
      "o=*";
};
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "true";
EOF

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
apt --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" update
apt --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" upgrade

# Move to testing.
cat <<EOF > /etc/apt/sources.list
deb http://deb.debian.org/debian/ testing main contrib non-free
deb-src http://deb.debian.org/debian/ testing main contrib non-free
deb http://security.debian.org/ testing/updates main contrib non-free
deb-src http://security.debian.org/ testing/updates main contrib non-free
EOF

# Yes, we need to run it this many times.
for i in {1..4}
do
  apt --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"  update
  apt --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"  full-upgrade
  apt --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"  autoremove
done

# Now install the apps that we guarantee to appear.
apt --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"  install git collectd unattended-upgrades
gsutil cp
gs://skia-push/debs/pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2017-03-02T16:55:37Z:38251c8ddc7f1033dd92064735aa45aedb48f527.deb pulld.deb
dpkg -i pulld.deb
systemctl start pulld.service

apt --assume-yes install --fix-broken

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
install -D --verbose --backup=none --group=root --owner=root --mode=600 collectd.conf /etc/collectd/collectd.conf
/etc/init.d/collectd restart


DEBCONF_DB_OVERRIDE="File{/tmp/override.dat readonly:true}" dpkg-reconfigure --priority=low --frontend=readline  unattended-upgrades

cat <<EOF > /etc/apt/apt.conf.d/50unattended-upgrades
Unattended-Upgrade::Origins-Pattern {
      "o=Debian,a=testing";
};
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "true";
EOF

# Check the output to confirm that the config is working.
unattended-upgrades -d


#!/bin/bash
#
# Setup the Swarming base image on Skia GCE instance.
#
# This script is used on a temporary GCE instance. Just run it on a fresh
# instance and then capture a snapshot of the disk. Any image
# started with this snapshot as its image should be immediately setup to
# run as a Swarming bot.
set -x -e
export TERM=xterm

echo 'Dpkg::Progress-Fancy "0";' | sudo tee /etc/apt/apt.conf.d/99progressbar

sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" update
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" upgrade

# Yes, we need to run it this many times.
for i in {1..4}
do
  sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" update
  sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" full-upgrade
  sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" autoremove
done

# Now install the apps that we guarantee to appear.
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install git collectd unattended-upgrades subversion make python-dev libfreetype6-dev xvfb python-twisted-core libpng-dev zlib1g-dev fontconfig libfontconfig-dev libglu-dev poppler-utils netpbm vim gyp g++ gdb unzip libgif-dev python-pil libosmesa-dev systemd libegl1-mesa-dev
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install gcc python-dev python-setuptools python-pip zip
sudo pip install -U crcmod

echo 'PATH="/home/default/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH"' >> ~/.bashrc
echo 'alias ll="ls -al"' >> ~/.bashrc

sudo rm -rf /etc/boto.cfg

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
LoadPlugin write_http

<Plugin write_http>
    <Node "desktop">
        URL "https://collectd.skia.org/collectd-post"
        Format "JSON"
   </Node>
</Plugin>
EOF
sudo install -D --verbose --backup=none --group=root --owner=root --mode=600 collectd.conf /etc/collectd/collectd.conf
sudo /etc/init.d/collectd restart

# Setup unattended upgrades. Limit cached packages to 1GB.
cat <<EOF | sudo tee /etc/apt/apt.conf.d/20auto-upgrades
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::MaxSize "1024";
EOF

cat <<EOF | sudo tee /etc/apt/apt.conf.d/50unattended-upgrades
Unattended-Upgrade::Origins-Pattern {
      "\${distro_id}:\${distro_codename}-security";
};
Unattended-Upgrade::Remove-Unused-Dependencies "true";
EOF

# Check the output to confirm that the config is working.
sudo unattended-upgrades -d --dry-run


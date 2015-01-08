#!/bin/bash
set -x

cd

# Install all the system level dependencies.
sudo apt-get install --assume-yes monit nginx collectd make

# Vars to use with 'install'.
PARAMS="-D --verbose --backup=none --group=default --owner=default --preserve-timestamps -T"
ROOT_PARAMS="-D --verbose --backup=none --group=root --owner=root --preserve-timestamps -T"
EXE_FILE="--mode=755"
CONFIG_FILE="--mode=666"
MONIT_CONFIG_FILE="--mode=600"

# Install pull.
# Temporary step to bootstrap monitoring using Skia Push.
gsutil cp gs://skia-push/debs/pull/pull:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-15T14:12:52Z:6152bc3bcdaa54989c957809e77bed282c35676b.deb pull.deb
sudo dpkg -i pull.deb

# Add the nginx configuration files.
cd ~/buildbot/compute_engine_scripts/monitoring/
sudo rm -f /etc/nginx/sites-enabled/default
sudo cp monitor_nginx /etc/nginx/sites-available/monitor
sudo rm -f /etc/nginx/sites-enabled/monitor
sudo ln -s /etc/nginx/sites-available/monitor /etc/nginx/sites-enabled/monitor
sudo cp alerts_nginx /etc/nginx/sites-available/alerts
sudo rm -f /etc/nginx/sites-enabled/alerts
sudo ln -s /etc/nginx/sites-available/alerts /etc/nginx/sites-enabled/alerts

# Create the directory for www logs if necessary.
mkdir -p /mnt/pd0/wwwlogs

# Now that the default installs are in place, overwrite the installs with our
# custom config files.
cd ~/buildbot/compute_engine_scripts/monitoring/
sudo install $PARAMS $CONFIG_FILE bashrc /home/default/.bashrc
sudo install $ROOT_PARAMS $CONFIG_FILE monitoring_monit /etc/monit/conf.d/monitoring
sudo install $ROOT_PARAMS $MONIT_CONFIG_FILE monitrc /etc/monit/monitrc
sudo install $ROOT_PARAMS $CONFIG_FILE collectd /etc/collectd/collectd.conf

# Confirm that monit is happy.
sudo monit -t
sudo monit reload

sudo /etc/init.d/collectd restart
sudo /etc/init.d/nginx restart

#!/bin/bash
set -x

cd

# Install all the system level dependencies.
sudo apt-get install --assume-yes monit squid3 collectd make

# Vars to use with 'install'.
PARAMS="-D --verbose --backup=none --group=default --owner=default --preserve-timestamps -T"
ROOT_PARAMS="-D --verbose --backup=none --group=root --owner=root --preserve-timestamps -T"
EXE_FILE="--mode=755"
CONFIG_FILE="--mode=666"

# Install Go
if [ -d go ]; then
  echo Go already installed.
else
  wget https://storage.googleapis.com/golang/go1.3.3.linux-amd64.tar.gz
  tar -xzf go1.3.3.linux-amd64.tar.gz
fi

mkdir=$HOME/golib
export GOROOT=$HOME/go
export GOPATH=$HOME/golib
export PATH=$PATH:$GOROOT/bin

# Install Node
NODE_VERSION="node-v0.10.33-linux-x64"
if [ -d ${NODE_VERSION} ]; then
  echo Node already installed.
else
  wget http://nodejs.org/dist/v0.10.33/${NODE_VERSION}.tar.gz
  tar xzf ${NODE_VERSION}.tar.gz
fi

export PATH=$PATH:$(pwd)/${NODE_VERSION}/bin

# Build applications
go get -u skia.googlesource.com/buildbot.git/perf/go/logserver \
  skia.googlesource.com/buildbot.git/monitoring/go/grains \
  skia.googlesource.com/buildbot.git/monitoring/go/prober \
  skia.googlesource.com/buildbot.git/monitoring/go/alertserver

# Install InfluxDB.
wget http://s3.amazonaws.com/influxdb/influxdb_latest_amd64.deb
sudo dpkg -i influxdb_latest_amd64.deb

# Install Grafana.
cd
sudo rm -rf grafana
GRAFANA=grafana-1.8.1
wget http://grafanarel.s3.amazonaws.com/${GRAFANA}.tar.gz
tar -xzf ${GRAFANA}.tar.gz
rm ${GRAFANA}.tar.gz
mv $GRAFANA grafana

# Build AlertServer.
cd $HOME/golib/src/skia.googlesource.com/buildbot.git/monitoring
make node_modules
make js

# Now that the default installs are in place, overwrite the installs with our
# custom config files.
cd ~/buildbot/compute_engine_scripts/monitoring/
sudo install $ROOT_PARAMS $EXE_FILE mlogserver_init /etc/init.d/logserver
sudo install $PARAMS $CONFIG_FILE influxdb-config.toml /opt/influxdb/shared/config.toml
sudo install $PARAMS $CONFIG_FILE bashrc /home/default/.bashrc
sudo install $PARAMS $CONFIG_FILE grafana-config.js /home/default/grafana/config.js
sudo install $ROOT_PARAMS $CONFIG_FILE monitoring_monit /etc/monit/conf.d/monitoring
sudo install $ROOT_PARAMS $EXE_FILE alertserver_init /etc/init.d/alertserver
sudo install $ROOT_PARAMS $EXE_FILE prober_init /etc/init.d/prober
sudo install $ROOT_PARAMS $EXE_FILE grains_init /etc/init.d/grains
sudo install $ROOT_PARAMS $CONFIG_FILE squid.conf /etc/squid3/squid.conf
sudo install $ROOT_PARAMS $CONFIG_FILE collectd /etc/collectd/collectd.conf

sudo /etc/init.d/monit -t
sudo /etc/init.d/monit restart
sudo /etc/init.d/influxdb restart
sudo /etc/init.d/logserver restart
sudo /etc/init.d/grains restart
sudo /etc/init.d/prober restart
sudo /etc/init.d/collectd restart
sudo /etc/init.d/squid3 restart
sudo /etc/init.d/alertserver restart

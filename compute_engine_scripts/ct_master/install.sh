#! /bin/bash
set -x

mkdir --parents /b/skia-repo/

# Add env vars to ~/.bashrc
echo 'export GOROOT=/usr/lib/go' >> ~/.bashrc
echo 'export GOPATH=/b/skia-repo/go/' >> ~/.bashrc
echo 'export PATH=$GOPATH/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

# Install necessary packages.
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install golang-go

# Checkout the Skia infra repo.
cd /b/skia-repo
go get -u -t go.skia.org/infra/...

# Start the CT poller.
cd /b/skia-repo/go/src/go.skia.org/infra/ct/
make all
nohup poller --log_dir=/b/storage/glog \
  --influxdb_host=https://metrics.skia.org \
  --influxdb_name=root \
  --influxdb_password="$(cat /b/storage/influxdb_password.txt)" \
  --influxdb_database=skmetrics &

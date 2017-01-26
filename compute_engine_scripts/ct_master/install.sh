#! /bin/bash
set -x

# Create required dirs.
mkdir --parents /b/skia-repo/
mkdir --parents /b/storage/

# Add env vars to ~/.bashrc
echo 'export GOROOT=/usr/lib/go' >> ~/.bashrc
echo 'export GOPATH=/b/skia-repo/go/' >> ~/.bashrc
echo 'export PATH=$GOPATH/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

# Install necessary packages.
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install golang-go python-django python-setuptools lua5.2
sudo easy_install -U pip
sudo pip install -U crcmod

# Checkout the Skia infra repo.
cd /b/skia-repo
GOPATH=/b/skia-repo/go/ go get -u -t go.skia.org/infra/...

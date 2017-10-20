#! /bin/bash
set -e
set -x

# Create required dirs.
mkdir --parents /b/skia-repo/
mkdir --parents /b/storage/

# Add env vars to ~/.bashrc
echo 'export GOROOT=/usr/local/go' >> ~/.bashrc
echo 'export GOPATH=/b/skia-repo/go/' >> ~/.bashrc
echo 'export PATH=$GOPATH/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

# Install necessary packages.
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install python-django python-setuptools lua5.2
sudo easy_install -U pip
sudo pip install -U crcmod

# Install golang.
GO_VERSION=go1.9.linux-amd64
wget https://storage.googleapis.com/golang/$GO_VERSION.tar.gz
tar -zxvf $GO_VERSION.tar.gz
sudo mv go /usr/local/$GO_VERSION
sudo ln -s /usr/local/$GO_VERSION /usr/local/go
sudo ln -s /usr/local/$GO_VERSION/bin/go /usr/bin/go
rm $GO_VERSION.tar.gz

# Checkout the Skia infra repo.
cd /b/skia-repo
GOPATH=/b/skia-repo/go/ go get -u -t go.skia.org/infra/...

# Create an ssh key for the autoscaler.
ssh-keygen -f ~/.ssh/google_compute_engine -t rsa -N ''

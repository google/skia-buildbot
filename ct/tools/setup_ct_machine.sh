#!/bin/bash
#
# Setup the files and checkouts on a cluster telemetry machine.
#

set -e

echo "Installing packages..."

# Install required packages.
sudo apt-get update;
sudo apt-get -y install python-django libgif-dev lua5.2 && \
    sudo easy_install -U pip && sudo pip install setuptools \
    --no-use-wheel --upgrade && sudo pip install -U crcmod

echo "Installing Python..."

# Install Python 2.7.11. See skbug.com/5562 for context.
sudo apt-get -y install autotools-dev blt-dev bzip2 dpkg-dev g++-multilib \
    gcc-multilib libbluetooth-dev libbz2-dev libexpat1-dev libffi-dev libffi6 \
    libffi6-dbg libgdbm-dev libgpm2 libncursesw5-dev libreadline-dev \
    libsqlite3-dev libssl-dev libtinfo-dev mime-support net-tools netbase \
    python-crypto python-mox3 python-pil python-ply quilt tk-dev zlib1g-dev \
    mesa-utils android-tools-adb
wget https://www.python.org/ftp/python/2.7.11/Python-2.7.11.tgz
tar xfz Python-2.7.11.tgz
cd Python-2.7.11/
./configure --prefix /usr/local/lib/python2.7.11 --enable-ipv6
make
sudo make install

echo "Checking out depot_tools..."

if [ ! -d "/b/depot_tools" ]; then
  cd /b/
  git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
  echo 'export PATH=/b/depot_tools:$PATH' >> ~/.bashrc
fi
PATH=$PATH:/b/depot_tools

echo "Checking out Chromium repository..."

mkdir -p /b/storage/chromium;
cd /b/storage/chromium;
/b/depot_tools/fetch chromium;
cd src;
git checkout master;
/b/depot_tools/gclient sync

echo "Checking out Skia's buildbot and trunk, and PDFium repositories..."

mkdir /b/skia-repo/;
cd /b/skia-repo/;
echo """
solutions = [
  { 'name'        : 'buildbot',
    'url'         : 'https://skia.googlesource.com/buildbot.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
  { 'name'        : 'trunk',
    'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
  { 'name'        : 'pdfium',
    'url'         : 'https://pdfium.googlesource.com/pdfium.git',
    'deps_file'   : 'DEPS',
    'managed'     : False,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
]
""" > .gclient;
/b/depot_tools/gclient sync;
# Checkout master in the repositories so that we can run "git pull" later.
cd buildbot;
git checkout master;
cd ../trunk;
git checkout master;
cd ../pdfium;
git checkout master;
# Create glog dir.
mkdir /b/storage/glog

echo
echo "The setup script has completed!"
echo

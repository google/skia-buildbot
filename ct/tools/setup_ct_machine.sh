#!/bin/bash
#
# Setup the files and checkouts on a cluster telemetry machine.
# WARNING: This script is out-of-date for non-swarming CT bots.
#

# Install required packages.
sudo apt-get update;
sudo apt-get -y install python-django libgif-dev lua5.2 && \
    sudo easy_install -U pip && sudo pip install setuptools \
    --no-use-wheel --upgrade && sudo pip install -U crcmod

# Checkout depot_tools
cd /b/
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
echo 'export PATH=/b/depot_tools:$PATH' >> ~/.bashrc

# Checkout Chromium repository.
mkdir -p /b/storage/chromium;
cd /b/storage/chromium;
/b/depot_tools/fetch chromium;
cd src;
git checkout master;
/b/depot_tools/gclient sync

# Checkout Skia's buildbot and trunk, and PDFium repositories.
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

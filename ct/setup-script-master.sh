#! /bin/bash
set -e
set -x

# Create required dirs.
mkdir --parents /b/storage/

# Install necessary packages.
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install python-django python-setuptools
sudo easy_install -U pip
sudo pip install -U crcmod

# Create an ssh key for the autoscaler.
ssh-keygen -f ~/.ssh/google_compute_engine -t rsa -N ''

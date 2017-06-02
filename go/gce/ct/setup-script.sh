#!/bin/bash

set -e

# Install packages.
sudo apt-get -y install mercurial libosmesa-dev npm nodejs-legacy libexpat1-dev:i386 clang-3.6 poppler-utils netpbm

sudo npm install -g npm@3.10.9
sudo npm install -g bower@1.6.5
sudo npm install -g polylint@2.4.3

sudo pip install coverage
sudo apt-get -y --purge remove apache2*
sudo sh -c "echo '* - nofile 500000' >> /etc/security/limits.conf"

# Install Chrome (for JS tests).
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
mkdir -p ~/.config/google-chrome
touch ~/.config/google-chrome/First\\ Run
sudo dpkg -i google-chrome-stable_current_amd64.deb || sudo apt-get -f -y install
rm google-chrome-stable_current_amd64.deb

# Fix depot_tools.
if [ ! -d depot_tools/.git ]; then
  rm -rf depot_tools
  git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
fi

# Fix symlinks.
sudo ln -s -f /usr/bin/clang-3.6 /usr/bin/clang
sudo ln -s -f /usr/bin/clang++-3.6 /usr/bin/clang++
sudo ln -s -f /usr/bin/llvm-cov-3.6 /usr/bin/llvm-cov
sudo ln -s -f /usr/bin/llvm-profdata-3.6 /usr/bin/llvm-profdata

# Bootstrap Swarming.
curl -sSf 'https://skia.googlesource.com/buildbot/+/master/ct/tools/setup_ct_machine.sh?format=TEXT' | base64 --decode > /tmp/ct_setup.sh
bash /tmp/ct_setup.sh

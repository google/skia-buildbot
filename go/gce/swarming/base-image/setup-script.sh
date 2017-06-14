#!/bin/bash
#
# Setups the instance image on Skia GCE instance.
#

set -e
set -x

sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" update
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install subversion git make python-dev libfreetype6-dev xvfb python-twisted-core libpng-dev zlib1g-dev fontconfig libfontconfig-dev libglu-dev poppler-utils netpbm vim gyp g++ gdb unzip linux-tools libgif-dev python-imaging libosmesa-dev linux-tools-3.16
sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install gcc python-dev python-setuptools
sudo easy_install -U pip
sudo pip install setuptools --no-binary :all: --upgrade
sudo pip install -U crcmod

echo 'PATH="/home/default/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH"' >> ~/.bashrc
echo 'alias ll="ls -al"' >> ~/.bashrc

# TODO(borenet): Why?
sudo rm -rf /etc/boto.cfg

# TODO(borenet): Do we need this?
#sudo cp /tmp/automount-sdb /etc/init.d/
#sudo chmod 755 /etc/init.d/automount-sdb
#sudo update-rc.d automount-sdb defaults
#sudo /etc/init.d/automount-sdb start

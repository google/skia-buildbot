#!/bin/sh
# Sets up the chrome-bot swarming user and installs python adb to /opt/adb

set -ex

rm /etc/localtime
ln -s /usr/share/zoneinfo/US/Eastern /etc/localtime

# This took a very long time for me.  Maybe just a fluke
apt-get update
apt-get install -y libusb-1.0-0-dev libssl-dev openssl \
                   time build-essential swig \
                   python-m2crypto ntpdate python-pip \
                   git android-tools-adb collectd \
                   python-setuptools \
                   --no-install-recommends
# Disable dhcp, otherwise RPIs can get assigned TWO IP addresses.
apt-get purge dhcpd5 \
              aptitude-common \
              aptitude || true

# Give the chrome-bot user access to various groups the pi user had access to.
#If chrome-bot is already a member, this won't hurt
for i in $(groups pi | cut -d " " -f 4-); do echo $i; adduser chrome-bot $i; done
gpasswd -a chrome-bot plugdev
# gpasswd -a chrome-bot adb

# Swarming requires a .boto file
touch /home/chrome-bot/.boto
chown chrome-bot:chrome-bot /home/chrome-bot/.boto

# Now to setup python-adb in /opt/adb
cwd=$(pwd)
cd /opt
if [ ! -e /usr/include/openssl/opensslconf.h ]
then
	sudo ln -s /usr/include/arm-linux-gnueabihf/openssl/opensslconf.h /usr/include/openssl/opensslconf.h
fi
pip install rsa
pip install wheel
pip install libusb1

if [ ! -f /opt/adb ]
then
	git clone https://github.com/google/python-adb
	./python-adb/make_tools.py
	ln python-adb/adb.zip adb
fi
cd $cwd

# Fix df
rm -f /etc/mtab
ln -s /proc/mounts /etc/mtab

# copy all the swarming related files.
mkdir -p /etc/swarming_config
cp oauth2_access_token_config.json /etc/swarming_config/oauth2_access_token_config.json
chmod 0644 /etc/swarming_config/oauth2_access_token_config.json
chown root:root /etc/swarming_config/oauth2_access_token_config.json

# add the script to bootstrap swarming.
cp bootstrap-swarming-bot          /opt/bootstrap-swarming-bot
chmod 0755 /opt/bootstrap-swarming-bot
chown root:root /opt/bootstrap-swarming-bot

# add the rc script to start and stop swarming.
cp swarming-rc-script /etc/init.d/swarming
chmod 0755 /etc/init.d/swarming
chown root:root /etc/init.d/swarming

# Make swarming run on boot
update-rc.d swarming defaults 90

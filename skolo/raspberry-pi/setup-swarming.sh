#!/bin/sh
# Sets up the chrome-bot swarming user and installs python adb to /opt/adb

set -ex

sudo chroot /opt/prod/root/

rm /etc/localtime
ln -s /usr/share/zoneinfo/US/Eastern /etc/localtime

# Give the chrome-bot user access to various groups the pi user had access to.
#If chrome-bot is already a member, this won't hurt
for i in $(groups pi | cut -d " " -f 4-); do echo $i; adduser chrome-bot $i; done
gpasswd -a chrome-bot plugdev
gpasswd -a chrome-bot adb

# Swarming requires a .boto file
touch /home/chrome-bot/.boto
chown chrome-bot:chrome-bot /home/chrome-bot/.boto

# This took a very long time for me.  Maybe just a fluke
apt-get update
apt-get install libusb-1.0-0-dev libssl-dev openssl time build-essential swig python-m2crypto ntpdate python-pip \
  git android-tools-adb collectd python3 --no-install-recommends
# Disable dhcp, otherwise RPIs can get assigned TWO IP addresses.
apt-get purge dhcpd5

# Now to setup python-adb in /opt/adb
cd /opt
if [ ! -e /usr/include/openssl/opensslconf.h ]
then
	sudo ln -s /usr/include/arm-linux-gnueabihf/openssl/opensslconf.h /usr/include/openssl/opensslconf.h
fi
pip install rsa
pip install libusb1

if [ ! -f /opt/adb ]
then
	git clone https://github.com/google/python-adb
	./python-adb/make_tools.py
	ln python-adb/adb.zip adb
fi

# Fix df
ln -s /proc/mounts /etc/mtab

# Make swarming run on boot
update-rc.d swarming defaults 90
# Adb can now be used by python /opt/adb
exit
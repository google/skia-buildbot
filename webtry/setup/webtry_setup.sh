#!/bin/bash
#
# Script to setup a GCE instance to run the webtry server.
# For full instructions see the README file.

function banner {
	echo ""
	echo "******************************************"
	echo "*"
	echo "* $1"
	echo "*"
	echo "******************************************"
	echo ""
}

banner "Installing debian packages needed for the server"

sudo apt-get install schroot debootstrap monit nginx nodejs nodejs-legacy mercurial

# wheezy nodejs install doesn't come with npm
[ -f /usr/bin/npm ] || curl https://www.npmjs.com/install.sh | sudo sh

# although aufs is being replaced by overlayfs, it's not clear
# to me if overlayfs is completely supported by schroot yet.
sudo apt-get install aufs-tools

banner "Setting up the webtry user account"
sudo adduser webtry

sudo mkdir /home/webtry/cache
sudo mkdir /home/webtry/cache/src
sudo mkdir /home/webtry/inout
sudo mkdir -p /tmp/wwwlogs
sudo chmod 777 /home/webtry/inout
sudo chmod 777 /home/webtry/cache
sudo chmod 777 /home/webtry/cache/src
sudo chmod 777 /tmp/weblogs

sudo cp sys/webtry_schroot /etc/schroot/chroot.d/webtry

CHROOT_JAIL=/srv/chroot/webtry_gyp
# Build the chroot environment.
if [ ! -d ${CHROOT_JAIL} ]; then
	banner "Building the chroot jail"
	sudo mkdir -p ${CHROOT_JAIL}

	sudo debootstrap --variant=minbase wheezy ${CHROOT_JAIL}
	sudo cp setup_jail.sh ${CHROOT_JAIL}/bin
	sudo chmod 755 ${CHROOT_JAIL}/bin/setup_jail.sh
	sudo chroot ${CHROOT_JAIL} /bin/setup_jail.sh
	sudo sh -c "echo 'none /dev/shm tmpfs rw,nosuid,nodev,noexec 0 0' >> ${CHROOT_JAIL}/etc/fstab"
	sudo sh -c "echo 'deb     http://http.debian.net/debian wheezy         contrib' >> ${CHROOT_JAIL}/etc/apt/sources.list"
	sudo sh -c "echo 'deb-src http://http.debian.net/debian wheezy         contrib' >> ${CHROOT_JAIL}/etc/apt/sources.list"

fi

gsutil cp gs://skia-push/debs/pull/pull:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-15T14:12:52Z:6152bc3bcdaa54989c957809e77bed282c35676b.deb pull.deb
sudo dpkg -i pull.deb
rm pull.deb


# The continue_install_jail script will update and build up the skia library
# inside the jail.

banner "Installing and updating software on the chroot jail"
sudo cp continue_install_jail.sh ${CHROOT_JAIL}/bin/continue_install_jail.sh
sudo chmod 755 ${CHROOT_JAIL}/bin/continue_install_jail.sh
sudo chroot ${CHROOT_JAIL} /bin/continue_install_jail.sh
sudo chown -R webtry:webtry ${CHROOT_JAIL}/skia_build/skia

sudo cp ../main.cpp ${CHROOT_JAIL}/skia_build/fiddle_main/
sudo cp ../seccmp_bpf.h ${CHROOT_JAIL}/skia_build/fiddle_main/
sudo cp ../fiddle_secwrap.cpp ${CHROOT_JAIL}/skia_build/fiddle_main/
sudo cp ../scripts/* ${CHROOT_JAIL}/skia_build/scripts/
sudo cp ../safec ${CHROOT_JAIL}/skia_build/scripts/
sudo cp ../safec++ ${CHROOT_JAIL}/skia_build/scripts/

# The continue_install script will fetch the latest versions of
# skia and depot_tools.  We split up the installation process into
# two pieces like this so that the continue_install script can
# be run independently of this one to fetch and build the latest skia.

banner "Building the webtry server outside the jail"

sudo cp continue_install.sh /home/webtry
sudo chown webtry:webtry /home/webtry/continue_install.sh
sudo su - webtry -c /home/webtry/continue_install.sh

banner "Setting up system initialization scripts"

ROOT_PARAMS="-D --verbose --backup=none --group=root --owner=root --preserve-timestamps -T"
EXE_FILE="--mode=755"
CONFIG_FILE="--mode=666"

sudo install $ROOT_PARAMS $EXE_FILE sys/webtry_init /etc/init.d/webtry
sudo install $ROOT_PARAMS $CONFIG_FILE sys/webtry_monit /etc/monit/conf.d/webtry

# Add the nginx configuration files.
sudo rm -f /etc/nginx/sites-enabled/default
sudo cp sys/webtry_nginx /etc/nginx/sites-available/webtry
sudo rm -f /etc/nginx/sites-enabled/webtry
sudo ln -s /etc/nginx/sites-available/webtry /etc/nginx/sites-enabled/webtry

# Download the SSL secrets from the metadata store.
CURL_CMD='curl -H "Metadata-Flavor: Google"'
META_PROJ_URL='http://metadata/computeMetadata/v1/project/attributes'

sudo mkdir -p /etc/nginx/ssl/
sudo sh <<CURL_SCRIPT
    $CURL_CMD $META_PROJ_URL/skfiddle-com-key -o /etc/nginx/ssl/skfiddle_com.key
    $CURL_CMD $META_PROJ_URL/skfiddle-com-pem -o /etc/nginx/ssl/skfiddle_com.pem
CURL_SCRIPT
sudo chmod 700 /etc/nginx/ssl
sudo sh -c "chmod 600 /etc/nginx/ssl/*"

# Confirm that monit is happy.
sudo monit -t
sudo monit reload

banner "Restarting webtry server"

sudo /etc/init.d/webtry restart
sudo /etc/init.d/nginx restart

banner "All done!"

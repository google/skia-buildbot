#!/bin/bash

set -e

# Install packages.

sudo apt-get -y install build-essential mercurial libosmesa-dev libexpat1-dev clang llvm poppler-utils netpbm gcc-multilib g++-multilib openjdk-8-jdk-headless libxi-dev python-django libc++-dev libc++abi-dev gperf bison usbutils

# TODO(borenet,rmistry): apt-get update is failing due to mismatch between
# cached apt packages files.
sudo rm -rf /var/lib/apt/lists/*

# Catapult requires a lsb-release file even if it's empty.
# TODO(rmistry): Remove this after https://github.com/catapult-project/catapult/issues/3705
# is resolved.
sudo touch /etc/lsb-release

# Obtain and symlink i386 libs.
sudo dpkg --add-architecture i386
sudo apt-get update
sudo apt-get -y install libfreetype6:i386 libfontconfig1:i386 libgl1-mesa-glx:i386 libglu1-mesa:i386 libx11-6:i386 libxext-dev:i386
sudo ln -sfn /usr/lib/i386-linux-gnu/libfreetype.so.6 /usr/lib/i386-linux-gnu/libfreetype.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libfontconfig.so.1 /usr/lib/i386-linux-gnu/libfontconfig.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libGLU.so.1 /usr/lib/i386-linux-gnu/libGLU.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libGL.so.1 /usr/lib/i386-linux-gnu/libGL.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libX11.so.6.3.0 /usr/lib/i386-linux-gnu/libX11.so

# MySQL setup.
sudo debconf-set-selections <<< 'mysql-server mysql-server/root_password password tmp_pass'
sudo debconf-set-selections <<< 'mysql-server mysql-server/root_password_again password tmp_pass'
sudo apt-get -y install mysql-client mysql-server
sudo mysql -uroot -ptmp_pass -e "SET PASSWORD = PASSWORD('');"
cat <<EOF | sudo tee --append /etc/mysql/my.cnf

[mysqld]
# Required to fix "Error 1709: Index column size too large. The maximum column size is 767 bytes."
character_set_server = latin1
collation_server = latin1_swedish_ci
EOF

# NodeJS / NPM.
curl -sL https://deb.nodesource.com/setup_6.x | sudo bash -
sudo apt-get -y install nodejs
sudo npm install -g npm@3.10.9
sudo npm install -g bower@1.6.5
sudo npm install -g polylint@2.10.4

# Python Coverage.
sudo pip install coverage

# Increase nofile limit.
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

# Fix file ownership (necessary for disks created from snapshot due to
# mismatched numerical user IDs and group IDs; harmless in other cases).
pushd /mnt/pd0/
ls | grep --invert-match 'lost+found' | \
  xargs --no-run-if-empty sudo chown --recursive chrome-bot:chrome-bot
popd

# Get access token from metadata.
TOKEN=`curl "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" -H "Metadata-Flavor: Google" | python -c "import sys, json; print json.load(sys.stdin)['access_token']"`
# Bootstrap Swarming.
sudo ln -s /mnt/pd0 /b
mkdir -p /b/s
SWARMING=https://chromium-swarm.appspot.com
if [[ $(hostname) == *"-i-"* ]]; then
  SWARMING=https://chrome-swarming.appspot.com
fi
HOSTNAME=`hostname`
curl ${SWARMING}/bot_code?bot_id=$HOSTNAME -H "Authorization":"Bearer $TOKEN" -o /b/s/swarming_bot.zip
ln -sfn /b/s /b/swarm_slave

cat <<EOF | sudo tee /etc/systemd/system/swarming_bot.service
[Unit]
Description=Swarming bot
After=network.target

[Service]
Type=simple
User=chrome-bot
Restart=on-failure
RestartSec=10
ExecStart=/usr/bin/env python /b/s/swarming_bot.zip start_bot

[Install]
WantedBy=default.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable swarming_bot.service

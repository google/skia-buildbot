#!/bin/bash

set -e

# Install packages.

sudo apt-get --assume-yes install build-essential mercurial libosmesa-dev libexpat1-dev clang llvm \
  poppler-utils netpbm gcc-multilib g++-multilib libxi-dev python-django python3-distutils \
  libc++-dev libc++abi-dev gperf bison usbutils libncurses5 locales

# Catapult requires a lsb-release file even if it's empty.
# TODO(rmistry): Remove this after https://github.com/catapult-project/catapult/issues/3705
# is resolved.
sudo touch /etc/lsb-release

# gcc-4.8 is only available in jessie. gcc-4.8 is required to compile for Ubuntu-14.04.
cat <<EOF | sudo tee --append /etc/apt/sources.list
deb http://cdn-fastly.deb.debian.org/debian/ jessie main
deb-src http://cdn-fastly.deb.debian.org/debian/ jessie main
deb http://security.debian.org/ jessie/updates main
deb-src http://security.debian.org/ jessie/updates main
EOF
sudo apt-get update
sudo apt-get --assume-yes install gcc-4.8 g++-4.8

# Buster locales need to be configured and set to avoid spurious bash
# complaints.
echo 'LC_ALL=en_US.UTF-8' | sudo tee --append /etc/environment
echo 'en_US.UTF-8 UTF-8' | sudo tee --append /etc/locale.gen
echo 'LANG=en_US.UTF-8' | sudo tee --append /etc/locale.conf
sudo locale-gen en_US.UTF-8

# Obtain and symlink i386 libs.
sudo dpkg --add-architecture i386
sudo apt-get update
sudo apt-get --assume-yes install libfreetype6:i386 libfontconfig1:i386 libgl1-mesa-glx:i386 \
  libglu1-mesa:i386 libx11-6:i386 libxext-dev:i386
# -sfn is --symbolic --force --no-dereference
sudo ln -sfn /usr/lib/i386-linux-gnu/libfreetype.so.6 /usr/lib/i386-linux-gnu/libfreetype.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libfontconfig.so.1 /usr/lib/i386-linux-gnu/libfontconfig.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libGLU.so.1 /usr/lib/i386-linux-gnu/libGLU.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libGL.so.1 /usr/lib/i386-linux-gnu/libGL.so
sudo ln -sfn /usr/lib/i386-linux-gnu/libX11.so.6.3.0 /usr/lib/i386-linux-gnu/libX11.so

# NodeJS / NPM.
# --location basically means follow redirects.
curl --silent --location https://deb.nodesource.com/setup_6.x | sudo bash -
sudo apt-get --assume-yes install nodejs npm
sudo npm install --global npm@3.10.9

# Python Coverage.
sudo pip install coverage
# Install psutil python module. See skbug.com/7328.
sudo pip install psutil

# Increase nofile limit.
echo '* - nofile 500000' | sudo tee --append /etc/security/limits.conf

# Install Chrome (for JS tests).
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
mkdir --parents ~/.config/google-chrome
touch ~/.config/google-chrome/First\\ Run
sudo dpkg --install google-chrome-stable_current_amd64.deb || \
  sudo apt-get --fix-broken --assume-yes install
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

# Install docker
pushd /tmp
  # gittiles makes it hard to download the raw file, so just download it from github.
  wget https://raw.githubusercontent.com/google/skia-buildbot/master/scripts/run_on_swarming_bots/install_docker.py
  # The script returns exit code 1 on success, because it's intended to reboot the swarming bot
  set +e
  python -u /tmp/install_docker.py
  set -e
popd

# Get access token from metadata.
TOKEN_URL="http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
TOKEN="$(curl "${TOKEN_URL}" --header "Metadata-Flavor: Google" | \
  python -c "import sys, json; print json.load(sys.stdin)['access_token']")"
# Bootstrap Swarming.
sudo ln --symbolic /mnt/pd0 /b
mkdir --parents /b/s
SWARMING=https://chromium-swarm.appspot.com
if [[ $(hostname) == *"-i-"* ]]; then
  SWARMING=https://chrome-swarming.appspot.com
elif [[ $(hostname) == *"-d-"* ]]; then
  SWARMING=https://chromium-swarm-dev.appspot.com
fi
HOSTNAME=`hostname`
curl "${SWARMING}/bot_code?bot_id=${HOSTNAME}" --header "Authorization":"Bearer $TOKEN" \
  --location --output /b/s/swarming_bot.zip

cat <<EOF | sudo tee /etc/systemd/system/swarming_bot.service
[Unit]
Description=Swarming bot
After=network.target

[Service]
Type=simple
User=chrome-bot
Restart=on-failure
RestartSec=10
ExecStart=/usr/bin/env python3 /b/s/swarming_bot.zip start_bot

[Install]
WantedBy=default.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable swarming_bot.service

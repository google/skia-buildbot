#!/bin/bash
#
# Setup the files and checkouts on a cluster telemetry machine.
#

set -e

# Install packages.
echo "Installing packages..."
sudo apt-get update
sudo apt-get -y install libosmesa-dev clang-3.6 poppler-utils netpbm \
    python3-django libgif-dev lua5.2 libnss3 python-setuptools python-pip \
    libglu1 libgtk3.0 xvfb gperf bison libglu1-mesa-dev libgbm-dev
sudo pip install -U crcmod mock psutil httplib2 numpy pandas

# Install openjdk-8. See skbug.com/6975 for context.
sudo apt-get -y install openjdk-8-jdk openjdk-8-jre

echo "Installing Python..."

sudo apt-get -y install autotools-dev blt-dev bzip2 dpkg-dev g++-multilib \
    gcc-multilib libbluetooth-dev libbz2-dev libexpat1-dev libffi-dev libffi6 \
    libffi6-dbg libgdbm-dev libgpm2 libncursesw5-dev libreadline-dev \
    libsqlite3-dev libssl-dev libtinfo-dev mime-support net-tools netbase \
    python-crypto python-mox3 python-pil python-ply quilt tk-dev zlib1g-dev \
    mesa-utils android-tools-adb python3-distutils

# Install python3.8 (skbug.com/12021). This will not be needed when all of CT
# uses python3 and we can use it via CIPD instead.
sudo apt install python3.8 -y
sudo rm /usr/bin/python3
sudo ln -s /usr/bin/python3.8 /usr/bin/python3

echo "Bring artifacts in from Google storage..."
# TODO(rmistry): Figure out which ones we really need.
/snap/bin/gsutil cp gs://cluster-telemetry-bucket/artifacts/bots/.gitconfig_ct ~/.gitconfig
/snap/bin/gsutil cp gs://cluster-telemetry-bucket/artifacts/bots/.netrc_ct ~/.netrc
/snap/bin/gsutil cp gs://cluster-telemetry-bucket/artifacts/bots/.boto_ct ~/.boto

echo "Checking out depot_tools..."
if [ ! -d ~/depot_tools ]; then
  cd ~
  git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
  echo 'export PATH=~/depot_tools:$PATH' >> ~/.bashrc
fi
PATH=$PATH:~/depot_tools

# To keep .boto file consistent with golo bots.
cp ~/.boto ~/.boto.puppet-bak

# CT uses this directory for storage of artifacts.
mkdir -p /b/storage

# If the bot is a builder then checkout Chromium and Skia repositories.
if [[ $(hostname -s) = ct-*-builder* ]]; then
  echo "Checking out Chromium repository..."
  mkdir -p /b/storage/chromium
  cd /b/storage/chromium
  if [[ $(hostname -s) = *android* ]]; then
    # Say yes to prompts for installing Android SDK.
    yes | ~/depot_tools/fetch android
  else
    ~/depot_tools/fetch chromium
  fi
  cd src
  git checkout master
  ~/depot_tools/gclient sync

  echo "Checking out Skia repository..."
  mkdir /b/skia-repo/
  cd /b/skia-repo/
  cat > .gclient << EOF
solutions = [
  { 'name'        : 'trunk',
    'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
]
EOF
  ~/depot_tools/gclient sync
  cd trunk
  git checkout master
fi

# Get access token from metadata.
TOKEN_URL="http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
TOKEN="$(curl "${TOKEN_URL}" --header "Metadata-Flavor: Google" | \
  python -c "import sys, json; print json.load(sys.stdin)['access_token']")"
# Bootstrap Swarming.
mkdir -p /b/s
SWARMING=https://chrome-swarming.appspot.com
HOSTNAME=`hostname`
curl "${SWARMING}/bot_code?bot_id=${HOSTNAME}" --header "Authorization":"Bearer $TOKEN" \
  --location --output /b/s/swarming_bot.zip

# See skbug.com/9425 for why LimitNOFILE is set.
cat <<EOF | sudo tee /etc/systemd/system/swarming_bot.service
[Unit]
Description=Swarming bot
After=network.target

[Service]
Type=simple
LimitNOFILE=50000
User=chrome-bot
Restart=on-failure
RestartSec=10
ExecStart=/usr/bin/env python3 /b/s/swarming_bot.zip start_bot

[Install]
WantedBy=default.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable swarming_bot.service

echo
echo "The setup script has completed!"
echo

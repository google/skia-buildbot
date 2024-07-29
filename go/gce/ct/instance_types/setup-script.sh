#!/bin/bash
#
# Setup the files and checkouts on a cluster telemetry machine.
#

set -e

echo "Installing required packages..."
sudo apt-get -y install bzip2

echo "Checking out depot_tools..."
if [ ! -d ~/depot_tools ]; then
  cd ~
  git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
  echo 'export PATH=~/depot_tools:$PATH' >> ~/.bashrc
fi
PATH=$PATH:~/depot_tools

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
  cat > .gclient << EOF
solutions = [
  { 'name'        : 'src',
    'url'         : 'https://chromium.googlesource.com/chromium/src.git',
    'managed'     : False,
    'custom_deps' : {},
    'custom_vars' : {
      'checkout_pgo_profiles': True,
    },
  },
]
EOF
  if [[ $(hostname -s) = *android* ]]; then
    echo -e "target_os = [ 'android' ]" >> .gclient
  fi
  cd src
  git checkout main
  ~/depot_tools/gclient sync

  # Install all build deps.
  cd build
  if [[ $(hostname -s) = *android* ]]; then
    sudo bash install-build-deps.sh --android
  else
    sudo bash install-build-deps.sh
  fi
fi

# TODO(rmistry): Figure out what packages are required by the masters
# and the workers.

# Get access token from metadata.
TOKEN_URL="http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
TOKEN="$(curl "${TOKEN_URL}" --header "Metadata-Flavor: Google" | \
  python3 -c "import sys, json; print(json.load(sys.stdin)['access_token'])")"
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

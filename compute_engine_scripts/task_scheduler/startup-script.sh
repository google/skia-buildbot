#! /bin/bash

# Install depot_tools if needed.
if [[ ! -d /mnt/pd0/depot_tools ]]; then
  sudo -u default git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git /mnt/pd0/depot_tools
fi

# Add depot_tools to PATH if needed.
if [ -z "$(which gclient)" ]; then
  sudo -u default echo '# Add depot_tools to PATH"' >> /home/default/.bashrc
  sudo -u default echo 'export PATH="/mnt/pd0/depot_tools:$PATH"' >> /home/default/.bashrc
fi

# Create tmp dir if needed.
sudo -u default mkdir -p /mnt/pd0/tmp

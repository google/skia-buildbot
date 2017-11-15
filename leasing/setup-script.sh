#! /bin/bash

pushd /home/default

# Install luci/client-py if needed.
if [[ ! -e client-py ]]; then
  git clone https://github.com/luci/client-py.git
  sudo -u default echo 'export PATH="/home/default/client-py:$PATH"' >> .bashrc
fi

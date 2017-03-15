#! /bin/bash

pushd /home/default

# Install repo tool if needed.
if [[ ! -e bin/repo ]]; then
  sudo -u default mkdir bin
  sudo -u default wget https://storage.googleapis.com/git-repo-downloads/repo -O bin/repo
  sudo -u default chmod a+x bin/repo
fi

# Install gcompute-tools if needed.
if [[ ! -d gcompute-tools ]]; then
  sudo -u default git clone https://gerrit.googlesource.com/gcompute-tools
fi

# Add repo and gcompute-tools to PATH if needed.
if [ -z "$(which repo)" ]; then
  sudo -u default echo '# Add Android tools to PATH"' >> .bashrc
  sudo -u default echo 'export PATH="/home/default/bin:$PATH"' >> .bashrc
  sudo -u default echo 'export PATH="/home/default/gcompute-tools:$PATH"' >> .bashrc
fi

popd

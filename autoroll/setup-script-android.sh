#! /bin/bash

pushd /home/default

# Install required packages.
sudo apt-get -y install gcc make

# Install repo tool if needed.
if [[ ! -e bin/repo ]]; then
  sudo -u default mkdir bin
  sudo -u default wget https://storage.googleapis.com/git-repo-downloads/repo -O bin/repo
  sudo -u default chmod a+x bin/repo
fi

# Add repo and gcompute-tools to PATH if needed.
if [ -z "$(which repo)" ]; then
  sudo -u default echo '# Add Android tools to PATH"' >> .bashrc
  sudo -u default echo 'export PATH="/home/default/bin:$PATH"' >> .bashrc
fi

# Set git configs required for the repo tool to not prompt.
sudo -u default git config --global color.ui true
sudo -u default git config --global user.email '31977622648@project.gserviceaccount.com'
sudo -u default git config --global user.name 'Skia_Android Canary Bot'

popd

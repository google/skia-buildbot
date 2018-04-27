#! /bin/bash

pushd /home/default

# Install necessary packages (from https://source.android.com/setup/initializing).
sudo apt-get update
sudo apt-get install openjdk-8-jdk
sudo apt-get install git-core gnupg flex bison gperf build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip

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

# Set git configs required for the repo tool to not prompt.
sudo -u default git config --global color.ui true

# Create a 200G ram disk for ccache.
mkdir /mnt/pd0/ccache
sudo mount -t tmpfs -o size=200G,nr_inodes=10M,mode=1777 tmpfs /mnt/pd0/ccache
sudo chown default:default -R /mnt/pd0/ccache
# Add mounting instructions to fstab so that the ram disk remounts on reboot.
echo "tmpfs /mnt/pd0/ccache tmpfs nodev,nosuid,noexec,nodiratime,size=200G 0 0" | sudo tee -a /etc/fstab

popd

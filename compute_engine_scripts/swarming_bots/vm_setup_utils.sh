#!/bin/bash
#
# Utility functions for the Skia GCE setup scripts.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


function install_packages {
  # Add new packages that are not yet part of the image below.
  echo
  echo "Install Required packages"
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
    "sudo debconf-set-selections <<< 'mysql-server mysql-server/root_password password tmp_pass' && " \
    "sudo debconf-set-selections <<< 'mysql-server mysql-server/root_password_again password tmp_pass' && " \
    "sudo apt-get -y install mercurial mysql-client mysql-server valgrind libosmesa-dev npm " \
    "  nodejs-legacy libexpat1-dev:i386 clang-3.6 poppler-utils netpbm && " \
    "sudo npm install -g npm@3.10.9 && " \
    "sudo npm install -g bower@1.6.5 && " \
    "sudo npm install -g polylint@2.4.3 && " \
    "mysql -uroot -ptmp_pass -e \"SET PASSWORD = PASSWORD('');\" && " \
    "wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && " \
    "mkdir -p ~/.config/google-chrome && touch ~/.config/google-chrome/First\ Run && " \
    "(sudo dpkg -i google-chrome-stable_current_amd64.deb || sudo apt-get -f -y install) && " \
    "rm google-chrome-stable_current_amd64.deb && " \
    "sudo pip install coverage" \
    || FAILED="$FAILED InstallPackages"
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
    "sudo apt-get -y --purge remove apache2* && " \
    "sudo sh -c \"echo '* - nofile 500000' >> /etc/security/limits.conf\" " \
    || FAILED="$FAILED RemoveApache2FixUlimit"
  echo
}

# TODO(borenet): Remove this function after we capture a new disk image.
function fix_depot_tools {
  echo
  echo "Fix depot_tools"
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
    "if [ ! -d depot_tools/.git ]; then rm -rf depot_tools; git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git; fi" \
    || FAILED="$FAILED FixDepotTools"
  echo
}

function setup_symlinks {
  # Add new symlinks that are not yet part of the image below.
  echo
  echo "Setup Symlinks"
   gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
     "sudo ln -s -f /usr/bin/clang-3.6 /usr/bin/clang && " \
     "sudo ln -s -f /usr/bin/clang++-3.6 /usr/bin/clang++ && " \
     "sudo ln -s -f /usr/bin/llvm-cov-3.6 /usr/bin/llvm-cov && " \
     "sudo ln -s -f /usr/bin/llvm-profdata-3.6 /usr/bin/llvm-profdata" \
     || FAILED="$FAILED InstallPackages"
  echo
}

function install_go {
  echo
  echo "Install Go"
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
      "wget https://storage.googleapis.com/golang/$GO_VERSION.tar.gz && " \
      "tar -zxvf $GO_VERSION.tar.gz && " \
      "sudo mv go /usr/local/$GO_VERSION && " \
      "sudo ln -s /usr/local/$GO_VERSION /usr/local/go && " \
      "sudo ln -s /usr/local/$GO_VERSION/bin/go /usr/bin/go && " \
      "rm $GO_VERSION.tar.gz" \
      || FAILED="$FAILED InstallGo"
  echo
}

function download_file {
  DOWNLOAD_URL="gs://skia-buildbots/artifacts"
  if [ "$VM_IS_INTERNAL" = 1 ]; then
    DOWNLOAD_URL="gs://skia-buildbots/artifacts_internal"
  fi
  echo
  echo "===== Downloading $1 ====="
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE chrome-bot@$INSTANCE_NAME -- \
    "gsutil cp gs://skia-buildbots/artifacts/$1 ~" \
    || FAILED="$FAILED Download $1"
  echo
}

function copy_files {
  echo
  echo "===== Copying over required files. ====="
  # TODO(rmistry): This was added because ~/.boto is part of the disk image.
  # It won't be next time the swarming bot image is captured, so remove this
  # line at that time.
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
    "rm -f .boto"

  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_BOTS[@]}; do
    download_file $REQUIRED_FILE
  done
  echo
}

function run_swarming_bootstrap {
  echo
  echo "===== Run Swarming Bootstrap ====="
  swarming="https://chromium-swarm.appspot.com"
  if [[ "$INSTANCE_NAME" = skia-i-* ]] || [[ "$INSTANCE_NAME" = ct-vm-* ]]; then
    swarming="https://chrome-swarming.appspot.com"
  fi
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE chrome-bot@$INSTANCE_NAME -- \
    "sudo chmod 777 /b && mkdir /b/s && " \
    "wget $swarming/bot_code -O /b/s/swarming_bot.zip && " \
    "ln -s /b/s /b/swarm_slave" \
    || FAILED="$FAILED SwarmingBootstrap"
  echo
}

function setup_ct_swarming_bot {
  echo
  echo "===== Run CT Bootstrap. ====="
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE chrome-bot@$INSTANCE_NAME -- \
    "curl -sSf 'https://skia.googlesource.com/buildbot/+/master/ct/tools/setup_ct_machine.sh?format=TEXT' | base64 --decode > '/tmp/ct_setup.sh' && " \
    "bash /tmp/ct_setup.sh" \
    || FAILED="$FAILED CTBootstrap"
  echo
}

function reboot {
  echo
  echo "===== Rebooting the instance ======"
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$INSTANCE_NAME -- \
    "sudo reboot" \
    || FAILED="$FAILED Reboot"
  echo
}

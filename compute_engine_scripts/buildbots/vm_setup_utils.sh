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
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo debconf-set-selections <<< 'mysql-server mysql-server/root_password password tmp_pass' && " \
    "sudo debconf-set-selections <<< 'mysql-server mysql-server/root_password_again password tmp_pass' && " \
    "sudo apt-get -y install mercurial mysql-client mysql-server valgrind libosmesa-dev npm " \
    "  nodejs-legacy libexpat1-dev:i386 clang-3.6 poppler-utils netpbm && " \
    "sudo npm install -g bower@1.6.5 && " \
    "sudo npm install -g polylint@2.4.3 && " \
    "mysql -uroot -ptmp_pass -e \"SET PASSWORD = PASSWORD('');\" && " \
    "wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && " \
    "mkdir -p ~/.config/google-chrome && touch ~/.config/google-chrome/First\ Run && " \
    "(sudo dpkg -i google-chrome-stable_current_amd64.deb || sudo apt-get -f -y install) && " \
    "rm google-chrome-stable_current_amd64.deb && " \
    "sudo pip install coverage" \
    || FAILED="$FAILED InstallPackages"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo apt-get -y --purge remove apache2* && " \
    "sudo sh -c \"echo '* - nofile 500000' >> /etc/security/limits.conf\" " \
    || FAILED="$FAILED RemoveApache2FixUlimit"
  echo
}

# TODO(borenet): Remove this function after we capture a new disk image.
function fix_depot_tools {
  echo
  echo "Fix depot_tools"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "if [ ! -d depot_tools/.git ]; then rm -rf depot_tools; git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git; fi" \
    || FAILED="$FAILED FixDepotTools"
  echo
}

function setup_symlinks {
  # Add new symlinks that are not yet part of the image below.
  echo
  echo "Setup Symlinks"
   $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
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
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
      "wget https://storage.googleapis.com/golang/$GO_VERSION.tar.gz && " \
      "tar -zxvf $GO_VERSION.tar.gz && " \
      "sudo mv go /usr/local/$GO_VERSION && " \
      "sudo ln -s /usr/local/$GO_VERSION /usr/local/go && " \
      "sudo ln -s /usr/local/$GO_VERSION/bin/go /usr/bin/go && " \
      "rm $GO_VERSION.tar.gz" \
      || FAILED="$FAILED InstallGo"
  echo
}

function setup_android_sdk {
  echo
  echo "===== Android SDK. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "mkdir $SKIA_REPO_DIR;" \
    "cd $SKIA_REPO_DIR && " \
    "sudo apt-get -y install libncurses5:i386 && " \
    "wget http://dl.google.com/android/adt/adt-bundle-linux-x86_64-20130729.zip && " \
    "if [[ -d adt-bundle-linux-x86_64-20130729 ]]; then rm -rf adt-bundle-linux-x86_64-20130729; fi && " \
    "unzip adt-bundle-linux-x86_64-20130729.zip && " \
    "if [[ -d android-sdk-linux ]]; then rm -rf android-sdk-linux; fi && " \
    "mv adt-bundle-linux-x86_64-20130729/sdk android-sdk-linux && " \
    "rm -rf adt-bundle-linux-x86_64-20130729 adt-bundle-linux-x86_64-20130729.zip && " \
    "echo \"y\" | android-sdk-linux/tools/android update sdk --no-ui --filter android-19 && " \
    "echo 'export ANDROID_SDK_ROOT=$SKIA_REPO_DIR/android-sdk-linux' >> /home/$PROJECT_USER/.bashrc" \
    || FAILED="$FAILED AndroidSDK"
  echo
}

function setup_nacl {
  NACL_PEPPER_VERSION="pepper_32"
  echo
  echo "===== Native Client SDK and NaClPorts. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "mkdir $SKIA_REPO_DIR;" \
    "cd $SKIA_REPO_DIR && " \
    "wget http://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/nacl_sdk.zip && " \
    "if [[ -d nacl_sdk ]]; then rm -rf nacl_sdk; fi && " \
    "unzip nacl_sdk.zip && " \
    "rm nacl_sdk.zip && " \
    "nacl_sdk/naclsdk update $NACL_PEPPER_VERSION && " \
    "echo 'export NACL_SDK_ROOT=$SKIA_REPO_DIR/nacl_sdk/$NACL_PEPPER_VERSION' >> /home/$PROJECT_USER/.bashrc" \
    || FAILED="$FAILED NativeClient"
  echo
}

function checkout_skia_repos {
  echo
  echo "Checkout Skia Buildbot code"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "mkdir $SKIA_REPO_DIR;" \
    "cd $SKIA_REPO_DIR && " \
    "~/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
    "~/depot_tools/gclient sync;" \
    || FAILED="$FAILED CheckoutSkiaBuildbot"
  echo
}

function copy_files {
  echo
  echo "===== Copying over required files. ====="
    for REQUIRED_FILE in ${REQUIRED_FILES_FOR_BOTS[@]}; do
      $GCOMPUTE_CMD push --ssh_user=chrome-bot $INSTANCE_NAME \
        $REQUIRED_FILE /home/chrome-bot/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/storage/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/storage/skia-repo/
    done
    # TODO(rmistry): This was added because ~/.boto is part of the disk image.
    # It won't be next time the buildbot image is captured, so remove this line
    # at that time.
    $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME "rm -f .boto"
  echo
}

function run_swarming_bootstrap {
  echo
  echo "===== Run Swarming Bootstrap ====="
  $GCOMPUTE_CMD ssh --ssh_user=chrome-bot $INSTANCE_NAME \
    "sudo chmod 777 /b && mkdir /b/s && " \
    "wget https://chromium-swarm.appspot.com/bot_code -O /b/s/swarming_bot.zip" \
    || FAILED="$FAILED SwarmingBootstrap"
  echo
}

function copy_ct_files {
  echo
  echo "===== Create CT storage dir to copy files into. ====="
  $GCOMPUTE_CMD ssh --ssh_user=chrome-bot $INSTANCE_NAME \
    "mkdir /b/storage" \
    || FAILED="$FAILED CTStorageDir"
  echo
  echo "===== Copying over CT required files. ====="
  $GCOMPUTE_CMD push --ssh_user=chrome-bot $INSTANCE_NAME \
    /tmp/.gitconfig /home/chrome-bot/
  $GCOMPUTE_CMD push --ssh_user=chrome-bot $INSTANCE_NAME \
    /tmp/.boto /home/chrome-bot/
  $GCOMPUTE_CMD push --ssh_user=chrome-bot $INSTANCE_NAME \
    /tmp/.netrc /home/chrome-bot/
  $GCOMPUTE_CMD push --ssh_user=chrome-bot $INSTANCE_NAME \
    /tmp/google_storage_token.data /b/storage/google_storage_token.data
  $GCOMPUTE_CMD push --ssh_user=chrome-bot $INSTANCE_NAME \
    /tmp/client_secret.json /b/storage/client_secret.json
  echo
}

function setup_ct_swarming_bot {
  echo
  echo "===== Run CT Bootstrap. ====="
  $GCOMPUTE_CMD ssh --ssh_user=chrome-bot $INSTANCE_NAME \
    "curl -sSf 'https://skia.googlesource.com/buildbot/+/master/ct/tools/setup_ct_machine.sh?format=TEXT' | base64 --decode > '/tmp/ct_setup.sh' && " \
    "bash /tmp/ct_setup.sh" \
    || FAILED="$FAILED CTBootstrap"
  echo
}

function setup_contab {
  echo
  echo "===== Setup Crontab. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "crontab $SKIA_REPO_DIR/buildbot/compute_engine_scripts/buildbots/cron-file.txt" \
    || FAILED="$FAILED SetupCrontab"
  echo
}

function reboot {
  echo
  echo "===== Rebooting the instance ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo reboot" \
    || FAILED="$FAILED Reboot"
  echo
}

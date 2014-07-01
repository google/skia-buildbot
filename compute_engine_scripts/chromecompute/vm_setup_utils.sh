#!/bin/bash
#
# Utility functions for the GCE chromecompute setup scripts.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


function install_packages {
  echo
  echo "Install Required packages"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo dpkg --add-architecture i386 && sudo apt-get update && " \
    "sudo apt-get -y install haveged python-django openjdk-7-jre-headless zlib1g-dev:i386 libgif-dev:i386 libpng12-dev:i386 fontconfig:i386 libgl1-mesa-dev:i386 libglu1-mesa-dev:i386 ccache g++-multilib libpoppler-cpp-dev libpoppler-cpp0:i386 && " \
    "sudo cp /usr/lib/i386-linux-gnu/libpng.so /usr/lib32/ && " \
    "sudo cp /usr/lib/i386-linux-gnu/libpng12.so.0 /usr/lib32/ && " \
    "sudo apt-get -y install libpng12-dev libgtk2.0-dev ant clang-3.4 openjdk-7-jdk realpath libqt4-dev-bin libqt4-core libqt4-gui libqt4-dev:i386 icewm && " \
    "sudo apt-get -y remove python-zope.interface && " \
    "sudo easy_install zope.interface" \
    || FAILED="$FAILED InstallPackages"
  echo
}

function setup_symlinks {
  echo
  echo "Setup Symlinks"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo ln -s -f /usr/bin/ccache /usr/local/bin/cc && " \
    "sudo ln -s -f /usr/bin/ccache /usr/local/bin/c++ && " \
    "sudo ln -s -f /usr/bin/ccache /usr/local/bin/gcc && " \
    "sudo ln -s -f /usr/bin/ccache /usr/local/bin/g++ && " \
    "sudo ln -s -f /usr/lib/i386-linux-gnu/libfontconfig.so.1 /usr/lib32/libfontconfig.so && " \
    "sudo ln -s -f /usr/lib/i386-linux-gnu/libfreetype.so.6 /usr/lib32/libfreetype.so && " \
    "sudo ln -s -f /usr/lib/x86_64-linux-gnu/libgif.so.4.1.6 /usr/lib/x86_64-linux-gnu/libgif.so && " \
    "sudo ln -s -f /usr/lib/x86_64-linux-gnu/mesa/libGL.so.1 /usr/lib/x86_64-linux-gnu/libGL.so && " \
    "sudo ln -s -f /usr/lib/x86_64-linux-gnu/libGLU.so.1 /usr/lib/x86_64-linux-gnu/libGLU.so && " \
    "sudo ln -s /usr/lib/x86_64-linux-gnu/libQtCore.so.4 /usr/lib/x86_64-linux-gnu/libQtCore.so && " \
    "sudo ln -s /usr/lib/x86_64-linux-gnu/libQtGui.so.4 /usr/lib/x86_64-linux-gnu/libQtGui.so && " \
    "sudo ln -s /usr/lib/x86_64-linux-gnu/libQtOpenGL.so.4 /usr/lib/x86_64-linux-gnu/libQtOpenGL.so && " \
    "sudo ln -s /home/default/storage/skia-repo /home/chrome-bot && " \
    "sudo ln -s /usr/lib/i386-linux-gnu/libpoppler-cpp.so.0 /usr/lib/i386-linux-gnu/libpoppler-cpp.so && " \
    "sudo rm /usr/bin/moc && sudo ln -s /usr/bin/moc-qt4 /usr/bin/moc && " \
    "sudo rm -rf /usr/bin/qmake && sudo ln -s /usr/bin/qmake-qt4 /usr/bin/qmake && " \
    "sudo ln -s /home/default/google-cloud-sdk/bin/gsutil /usr/local/bin/gsutil" \
    || FAILED="$FAILED InstallPackages"
  echo
}

function setup_android_sdk {
  echo
  echo "===== Android SDK. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
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
    "mkdir $SKIA_REPO_DIR && " \
    "cd $SKIA_REPO_DIR && " \
    "~/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
    "~/depot_tools/gclient sync;" \
    || FAILED="$FAILED CheckoutSkiaBuildbot"
  echo

  echo
  echo "Checkout Skia Trunk code"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "cd $SKIA_REPO_DIR && " \
    "sed -i '$ d' .gclient && sed -i '$ d' .gclient && " \
    "echo \"\"\"
    { 'name'        : 'skia',
      'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
      'managed'     : True,
      'custom_deps' : {
      },
      'safesync_url': '',
    },
  ]
  \"\"\" >> .gclient && ~/depot_tools/gclient sync;" \
    || FAILED="$FAILED CheckoutSkiaTrunk"
  echo
}

function setup_crontab {
  echo
  echo "===== Setting up launch-on-reboot ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "python $SKIA_REPO_DIR/buildbot/scripts/launch_on_reboot.py"
    || FAILED="$FAILED LaunchOnReboot"
  echo
}

function fix_gsutil_path {
  echo
  echo "===== Fixing gsutil path ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo rm /usr/local/bin/gsutil && " \
    "sudo ln -s /home/default/google-cloud-sdk/platform/gsutil/gsutil /usr/local/bin/gsutil" \
     || FAILED="$FAILED FixGsutilPath"
  echo
}

function copy_files {
  echo
  echo "===== Copying over required files. ====="
    for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SKIA_BOTS[@]}; do
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/storage/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/storage/skia-repo/
    done
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

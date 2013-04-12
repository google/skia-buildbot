#!/bin/bash
#
# Setup all the slave buildbot instances.
#
# Copyright 2012 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SLAVES[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

for VM in $VM_SLAVE_NAMES; do
  VM_COMPLETE_NAME="${VM_NAME_BASE}-${VM}-${ZONE_TAG}"

  echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

  FAILED=""
  SLAVE_DIR=skia-slave

  echo "Install packages, Part 1."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo apt-get update && " \
    "sudo apt-get install --assume-yes libfreetype6 libfreetype6-dev" \
    || FAILED="$FAILED InstallPackages1"
  echo

  echo "Install packages, Part 2."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo apt-get update && " \
    "sudo apt-get install --assume-yes libgd2-xpm:i386" \
    || FAILED="$FAILED InstallPackages2"
  echo

  echo "Install packages, Part 3."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo apt-get update && " \
    "sudo apt-get install --assume-yes ia32-libs-multiarch " \
    "gcc-multilib g++-multilib lib32z1" \
    || FAILED="$FAILED InstallPackages3"
  echo

  echo "Install packages, Part 4."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo apt-get update && " \
    "sudo apt-get install --assume-yes openjdk-7-jdk libpng12-0 libpng12-dev libgl1-mesa-dev " \
    "libgl1-mesa-dri subversion postfix libgl1-mesa-glx libglu1-mesa libglu1-mesa-dev mesa-common-dev " \
    "libosmesa6 libosmesa6-dev doxygen clang libQt4-dev git make python-dev g++ python-epydoc " \
    "libfontconfig-dev unzip ant ccache libgif-dev libgif4:i386 && " \
    "sudo easy_install --upgrade google-api-python-client && " \
    "sudo easy_install --upgrade pyOpenSSL" \
    || FAILED="$FAILED InstallPackages4"
  echo

  echo "Setup symlinks."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo ln -s /usr/lib/i386-linux-gnu/libfreetype.so.6.8.0 /usr/lib32/libfreetype.so && " \
    "sudo ln -s /usr/lib/i386-linux-gnu/libfontconfig.so.1 /usr/lib32/libfontconfig.so && " \
    "sudo ln -s /usr/lib/i386-linux-gnu/mesa/libGL.so.1 /usr/lib32/libGL.so && " \
    "sudo ln -s /usr/lib/i386-linux-gnu/libGLU.so.1 /usr/lib32/libGLU.so && " \
    "sudo ln -s /usr/lib/i386-linux-gnu/libX11.so.6.3.0 /usr/lib32/libX11.so && " \
    "sudo ln -s /usr/lib32/libz.so.1 /usr/lib32/libz.so && " \
    "sudo ln -s /lib/i386-linux-gnu/libpng12.so.0 /usr/lib32/libpng.so && " \
    "sudo ln -s /usr/lib/i386-linux-gnu/libgif.so.4.1.6 /usr/lib/i386-linux-gnu/libgif.so && " \
    "sudo ln -s /usr/bin/ccache /usr/local/bin/cc && " \
    "sudo ln -s /usr/bin/ccache /usr/local/bin/c++ && " \
    "sudo ln -s /usr/bin/ccache /usr/local/bin/gcc && " \
    "sudo ln -s /usr/bin/ccache /usr/local/bin/g++ && " \
    "sudo update-alternatives --set java /usr/lib/jvm/java-7-openjdk-amd64/jre/bin/java" \
    || FAILED="$FAILED SetupSymlinks"
  echo

  echo "Mount the persistent disk."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "mkdir -p $SLAVE_DIR && " \
    "if [[ ! \$(cat /proc/mounts | grep $SLAVE_DIR) ]]; then sudo /usr/share/google/safe_format_and_mount -m \"mkfs.ext4 -F\" /dev/sdb /home/$PROJECT_USER/$SLAVE_DIR; fi && " \
    "sudo chmod 777 -R $SLAVE_DIR && " \
    "sudo ln -s /home/default/skia-slave /home/chrome-bot" \
    || FAILED="$FAILED MountDisk"
  echo

  echo "Check out depot tools."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "cd $SLAVE_DIR && " \
    "svn checkout http://src.chromium.org/svn/trunk/tools/depot_tools" \
    || FAILED="$FAILED DepotTools"
  echo

  echo "Check out the buildbot scripts."
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "cd $SLAVE_DIR && " \
    "if [[ -e /etc/boto.cfg ]]; then sudo mv /etc/boto.cfg /etc/boto.cfg.bak; fi && " \
    "/home/$PROJECT_USER/$SLAVE_DIR/depot_tools/gclient config https://skia.googlecode.com/svn/buildbot && " \
    "/home/$PROJECT_USER/$SLAVE_DIR/depot_tools/gclient sync" \
    || FAILED="$FAILED BuildbotScripts"
  echo

  echo "Android SDK"
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "cd $SLAVE_DIR && " \
    "wget http://dl.google.com/android/adt/adt-bundle-linux-x86_64-20130219.zip && " \
    "if [[ -d adt-bundle-linux-x86_64-20130219 ]]; then rm -rf adt-bundle-linux-x86_64-20130219; fi && " \
    "unzip adt-bundle-linux-x86_64-20130219.zip && " \
    "if [[ -d android-sdk-linux ]]; then rm -rf android-sdk-linux; fi && " \
    "mv adt-bundle-linux-x86_64-20130219/sdk android-sdk-linux && " \
    "rm -rf adt-bundle-linux-x86_64-20130219 adt-bundle-linux-x86_64-20130219.zip && " \
    "android-sdk-linux/tools/android update sdk --no-ui --filter android-15" \
    || FAILED="$FAILED AndroidSDK"
  echo

  echo "Native Client SDK and NaClPorts"
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "cd $SLAVE_DIR && " \
    "wget http://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/nacl_sdk.zip && " \
    "if [[ -d nacl_sdk ]]; then rm -rf nacl_sdk; fi && " \
    "unzip nacl_sdk.zip && " \
    "rm nacl_sdk.zip && " \
    "nacl_sdk/naclsdk update pepper_25 && " \
    "export NACL_SDK_ROOT=/home/$PROJECT_USER/$SLAVE_DIR/nacl_sdk/pepper_25 && " \
    "mkdir -p naclports && " \
    "cd naclports && " \
    "/home/$PROJECT_USER/$SLAVE_DIR/depot_tools/gclient config http://naclports.googlecode.com/svn/trunk/src && " \
    "/home/$PROJECT_USER/$SLAVE_DIR/depot_tools/gclient sync --delete_unversioned_trees --force && " \
    "cd src/libraries/zlib && " \
    "export NACL_PACKAGES_BITSIZE=32 && ./nacl-zlib.sh && " \
    "export NACL_PACKAGES_BITSIZE=64 && ./nacl-zlib.sh && " \
    "cd ../libpng && " \
    "export NACL_PACKAGES_BITSIZE=32 && ./nacl-libpng.sh && " \
    "export NACL_PACKAGES_BITSIZE=64 && ./nacl-libpng.sh " \
    || FAILED="$FAILED NativeClient"
  echo

  echo "Setup .bashrc"
  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "if [[ ! -f /home/$PROJECT_USER/.bashrc.bak ]]; then cp /home/$PROJECT_USER/.bashrc /home/$PROJECT_USER/.bashrc.bak; fi && " \
    "echo 'export PATH=\"/home/$PROJECT_USER/$SLAVE_DIR/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH\"' >> /home/$PROJECT_USER/.bashrc &&" \
    "echo 'alias s=\"cd /home/$PROJECT_USER/$SLAVE_DIR/buildbot/slave\"' >> /home/$PROJECT_USER/.bashrc && " \
    "echo 'export ANDROID_SDK_ROOT=/home/$PROJECT_USER/$SLAVE_DIR/android-sdk-linux' >> /home/$PROJECT_USER/.bashrc &&" \
    "echo 'export NACL_SDK_ROOT=/home/$PROJECT_USER/$SLAVE_DIR/nacl_sdk/pepper_25' >> /home/$PROJECT_USER/.bashrc" \
    || FAILED="$FAILED SetupBashrc"
  echo

  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SLAVES[@]}; do
    $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/default/$SLAVE_DIR/buildbot/
    $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/default/$SLAVE_DIR/buildbot/site_config/
    $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/default/$SLAVE_DIR/buildbot/third_party/chromium_buildbot/site_config/
  done

  if [[ $FAILED ]]; then
    echo
    echo "FAILURES: $FAILED"
    echo "Please manually fix these errors."
    echo
  fi

  echo """
Please manually ssh into ${VM_COMPLETE_NAME} and:
  * Generate SSH keys (ssh-keygen -t dsa).
  * Add the public key to the slave's /home/$PROJECT_USER/.ssh/authorized_keys
  * Add the public key to the buildbot master's /home/$PROJECT_USER/.ssh/authorized_keys

ssh cmd: ${GCOMPUTE_CMD} ssh --ssh_user=default ${VM_COMPLETE_NAME}
"""

done


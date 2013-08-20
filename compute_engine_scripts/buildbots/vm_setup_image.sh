#!/bin/bash
#
# Setup the Skia buildbot image on a GCE instance.
#
# The instance must be called "skia-buildbot-image". Create it with:
# gcutil --project=google.com:skia-buildbots addinstance skia-buildbot-image \
#        --zone=us-central1-b --service_account=default \
#        --service_account_scopes="https://www.googleapis.com/auth/devstorage.full_control" \
#        --network=default --external_ip_address=173.255.114.239 --print_json \
#        --machine_type=n1-standard-2-d \
#        --image=projects/debian-cloud/global/images/debian-7-wheezy-v20130723 \
#        --nopersistent_boot_disk
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SLAVES[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

VM_COMPLETE_NAME="skia-buildbot-image"

echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

FAILED=""

echo
echo "===== Add i386 architecture. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo dpkg --add-architecture i386 && sudo apt-get update" \
  || FAILED="$FAILED InstallPackages1"
echo

echo
echo "===== Install packages, Part 1. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo apt-get install --assume-yes libfreetype6 libfreetype6-dev" \
  || FAILED="$FAILED InstallPackages1"
echo

echo
echo "===== Install packages, Part 2. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo apt-get install --assume-yes libgd2-xpm libgd2-xpm:i386" \
  || FAILED="$FAILED InstallPackages2"
echo

echo
echo "===== Install packages, Part 3. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo apt-get install --assume-yes ia32-libs libgif4 libgif-dev openjdk-7-jdk " \
  "gcc-multilib g++-multilib lib32z1 && " \
  "sudo mv /usr/lib/libgif.* /usr/lib/x86_64-linux-gnu/ && " \
  "sudo mv /usr/lib/jvm/java-7-openjdk-amd64 /tmp/java-7-openjdk-amd64" \
  || FAILED="$FAILED InstallPackages3"
echo

echo
echo "===== Install packages, Part 4. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo apt-get install --assume-yes libpng12-0 libpng12-dev libgl1-mesa-dev " \
  "libgl1-mesa-dri subversion postfix libgl1-mesa-glx libglu1-mesa libglu1-mesa-dev mesa-common-dev " \
  "libosmesa6 libosmesa6-dev doxygen clang libQt4-dev git vim make python-dev g++ python-epydoc " \
  "libfontconfig-dev unzip ant ccache realpath python-setuptools bzip2 libgif4:i386 libgif-dev:i386 && " \
  "sudo easy_install --upgrade google-api-python-client && " \
  "sudo easy_install --upgrade pyOpenSSL" \
  || FAILED="$FAILED InstallPackages4"
echo

echo
echo "===== Setup symlinks. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo ln -s -f /usr/lib/i386-linux-gnu/libfreetype.so.6.8.1 /usr/lib32/libfreetype.so && " \
  "sudo ln -s -f /usr/lib/i386-linux-gnu/libfontconfig.so.1 /usr/lib32/libfontconfig.so && " \
  "sudo ln -s -f /usr/lib/i386-linux-gnu/libGL.so.1 /usr/lib32/libGL.so && " \
  "sudo ln -s -f /usr/lib/i386-linux-gnu/libGLU.so.1 /usr/lib32/libGLU.so && " \
  "sudo ln -s -f /usr/lib/i386-linux-gnu/libX11.so.6.3.0 /usr/lib32/libX11.so && " \
  "sudo ln -s -f /usr/lib32/libz.so.1 /usr/lib32/libz.so && " \
  "sudo ln -s -f /lib/i386-linux-gnu/libpng12.so.0 /usr/lib32/libpng.so && " \
  "sudo ln -s -f /usr/lib/i386-linux-gnu/libgif.so.4.1.6 /usr/lib/i386-linux-gnu/libgif.so && " \
  "sudo ln -s -f /usr/bin/ccache /usr/local/bin/cc && " \
  "sudo ln -s -f /usr/bin/ccache /usr/local/bin/c++ && " \
  "sudo ln -s -f /usr/bin/ccache /usr/local/bin/gcc && " \
  "sudo ln -s -f /usr/bin/ccache /usr/local/bin/g++ && " \
  "sudo mv /tmp/java-7-openjdk-amd64 /usr/lib/jvm/java-7-openjdk-amd64 && " \
  "sudo update-alternatives --set java /usr/lib/jvm/java-7-openjdk-amd64/jre/bin/java && " \
  "sudo ln -s /usr/lib/jvm/java-7-openjdk-amd64/bin/javac /usr/bin/javac && " \
  "sudo ln -s -f /home/default/skia-repo /home/chrome-bot" \
  || FAILED="$FAILED SetupSymlinks"
echo

echo
echo "===== Setup automount scripts. ====="
$GCOMPUTE_CMD push --ssh_user=root $VM_COMPLETE_NAME \
  image-files/automount-sdb /etc/init.d/
$GCOMPUTE_CMD push --ssh_user=root $VM_COMPLETE_NAME \
  image-files/automount-swap /etc/init.d/
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "sudo chmod 755 /etc/init.d/automount-sdb && " \
  "sudo update-rc.d automount-sdb defaults && " \
  "sudo /etc/init.d/automount-sdb start && " \
  "sudo chmod 755 /etc/init.d/automount-swap && " \
  "sudo update-rc.d automount-swap defaults && " \
  "sudo /etc/init.d/automount-swap start" \
  || FAILED="$FAILED SetupAutomountScript"
echo

echo
echo "===== Setup .bashrc ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
  "if [[ ! -f /home/$PROJECT_USER/.bashrc.bak ]]; then cp /home/$PROJECT_USER/.bashrc /home/$PROJECT_USER/.bashrc.bak; fi && " \
  "echo 'export PATH=\"/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH\"' >> /home/$PROJECT_USER/.bashrc && " \
  "echo 'alias m=\"cd /home/$PROJECT_USER/$SKIA_REPO_DIR/buildbot/master\"' >> /home/$PROJECT_USER/.bashrc && " \
  "echo 'alias s=\"cd /home/$PROJECT_USER/$SKIA_REPO_DIR/buildbot/slave\"' >> /home/$PROJECT_USER/.bashrc && " \
  "echo 'alias ll=\"ls -l\"' >> /home/$PROJECT_USER/.bashrc && " \
  "echo 'export ANDROID_SDK_ROOT=/home/$PROJECT_USER/$SKIA_REPO_DIR/android-sdk-linux' >> /home/$PROJECT_USER/.bashrc &&" \
  "echo 'export NACL_SDK_ROOT=/home/$PROJECT_USER/$SKIA_REPO_DIR/nacl_sdk/pepper_25' >> /home/$PROJECT_USER/.bashrc &&" \
  "echo 'source /home/$PROJECT_USER/.bashrc' >> /home/$PROJECT_USER/.bash_profile" \
  || FAILED="$FAILED SetupBashrc"
echo

if [[ $FAILED ]]; then
  echo
  echo "FAILURES: $FAILED"
  echo "Please manually fix these errors."
  echo
fi

echo
echo "===== Setup ssh keys. ====="
echo """
Please manually ssh into ${VM_COMPLETE_NAME} and:
  * Generate SSH keys (ssh-keygen -t dsa).
  * Add the public key to the machine's /home/$PROJECT_USER/.ssh/authorized_keys

ssh cmd: ${GCOMPUTE_CMD} ssh --ssh_user=default ${VM_COMPLETE_NAME}
"""

echo
echo "===== Setup emailing. ====="
echo """
Follow the instructions in
https://developers.google.com/compute/docs/networking#mailserver using skia.buildbots@gmail.com

ssh cmd: ${GCOMPUTE_CMD} ssh --ssh_user=$PROJECT_USER ${VM_COMPLETE_NAME}
"""
echo

echo
echo "Capture an image of this machine:"
echo "sudo python /usr/share/imagebundle/image_bundle.py -r / -o /home/default/skia-repo/ --log_file=/tmp/image.log"
echo "Copy the image to Google Storage:"
echo "* gsutil config"
echo "* gsutil cp /home/default/skia-repo/*.image.tar.gz gs://skia-images/skia-buildbot.image.tar.gz"
echo "Register the image with GCE:"
echo "* $GCOMPUTE_CMD deleteimage skia-buildbot-image"
echo "* $GCOMPUTE_CMD addimage skia-buildbot-image gs://skia-images/skia-buildbot.image.tar.gz --preferred_kernel=projects/google/global/kernels/gce-v20130603"
echo


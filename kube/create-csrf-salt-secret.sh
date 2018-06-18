#/bin/bash

# Creates the salt used to protect all applications from csrf attacks.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

cd /tmp/ramdisk
head -c 32 /dev/urandom > salt.txt
kubectl create secret generic csrf-salt --from-file=salt.txt
cd -

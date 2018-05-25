#/bin/bash

# Creates the salt used to protect debugger from csrf attacks.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

cd /tmp/ramdisk

head -c 50 /dev/urandom | base64 > salt.txt

kubectl create secret generic debugger-csrf-salt --from-file=salt.txt

cd -

#/bin/bash

# Creates the skia-corp-kube-config secret.

set -e -x
source ../../kube/corp-config.sh
source ../../bash/ramdisk.sh

if [ $# -ne 1 ]; then
  echo "The argument must be the skia-corp kube config."
  echo ""
  echo "./create-skia-corp-kube-config-secret.sh xyz"
  exit 1
fi

SECRET_VALUE=$1
SECRET_NAME="skia-corp-kube-config"
ORIG_WD=$(pwd)

cd /tmp/ramdisk
cat ${SECRET_VALUE} >> kube_config
kubectl create secret generic "${SECRET_NAME}" --from-file=kube_config

cd -

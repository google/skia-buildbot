#/bin/bash

# Creates the google-shaderc-github-token secret.

set -e -x
source ../../kube/config.sh
source ../../bash/ramdisk.sh

if [ "$#" -ne 1 ]; then
  echo "The argument must be the github token."
  echo ""
  echo "./create-google-shaderc-github-token-secret.sh xyz"
  exit 1
fi

SECRET_VALUE=$1
SECRET_NAME="google-shaderc-github-token"
ORIG_WD=$(pwd)

cd /tmp/ramdisk
echo ${SECRET_VALUE} >> github_token
kubectl create secret generic "${SECRET_NAME}" --from-file=github_token

cd -

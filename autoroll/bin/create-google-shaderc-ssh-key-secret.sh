#/bin/bash

# Creates the google-shaderc-ssh-key secret.

set -e -x
source ../../kube/config.sh

if [ "$#" -ne 1 ]; then
  echo "The argument must point to the id_rsa file."
  echo ""
  echo "./create-google-shaderc-ssh-key-secret.sh "
  exit 1
fi

SECRET_LOCATION=$1
SECRET_NAME="google-shaderc-ssh-key"

kubectl create secret generic "${SECRET_NAME}" --from-file=${SECRET_LOCATION}

cd -

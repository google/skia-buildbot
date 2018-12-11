#/bin/bash

# Creates the flutter-engine-ssh-key secret.

set -e -x
source ../../kube/config.sh

if [ "$#" -ne 1 ]; then
  echo "The argument must point to the id_rsa file."
  echo ""
  echo "./create-flutter-engine-ssh-key-secret.sh "
  exit 1
fi

SECRET_LOCATION=$1
SECRET_NAME="flutter-engine-ssh-key"

kubectl create secret generic "${SECRET_NAME}" --from-file=${SECRET_LOCATION}

cd -

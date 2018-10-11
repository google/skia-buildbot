#/bin/bash

# Creates the flutter-ssh-key secret.

set -e -x
source ./config.sh

if [ "$#" -ne 1 ]; then
  echo "The argument must point to the id_rsa file."
  echo ""
  echo "./create-flutter-ssh-key-secret.sh "
  exit 1
fi

SECRET_LOCATION=$1
SECRET_NAME="flutter-ssh-key"

kubectl create secret generic "${SECRET_NAME}" --from-file=${SECRET_LOCATION}

cd -

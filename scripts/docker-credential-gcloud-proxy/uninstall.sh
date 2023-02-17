#!/bin/bash

if [ $# -ne 1 ]; then
    echo "$0 <machine>"
    exit 1
fi

MACHINE=$1

ssh $MACHINE "
  sudo rm /usr/bin/docker-credential-gcloud &&
  sudo rm /docker-credential-gcloud-proxy.log &&
  sudo ln -s /usr/lib/google-cloud-sdk/bin/docker-credential-gcloud /usr/bin/docker-credential-gcloud
"

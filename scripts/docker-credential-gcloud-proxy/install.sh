#!/bin/bash

if [ $# -ne 1 ]; then
    echo "$0 <machine>"
    exit 1
fi

MACHINE=$1

scp docker-credential-gcloud-proxy.py "${MACHINE}:/home/chrome-bot"

ssh $MACHINE "
  sudo rm /usr/bin/docker-credential-gcloud &&
  sudo mv /home/chrome-bot/docker-credential-gcloud-proxy.py /usr/bin/docker-credential-gcloud &&
  sudo chmod ugo+rx /usr/bin/docker-credential-gcloud &&
  sudo touch /docker-credential-gcloud-proxy.log &&
  sudo chmod ugo+rw /docker-credential-gcloud-proxy.log
"

#!/bin/bash

set -x
{
WORKDIR=/mnt/pd0/buildbot_db_backup
mkdir -p $WORKDIR
pushd $WORKDIR
curl http://localhost:8001/backup > buildbot.db && \
sudo gsutil cp buildbot.db gs://skia-buildbots/db_backup/$(date +%Y/%m/%d)/buildbot.db
popd
rm -rf $WORKDIR
} > /var/log/logserver/buildbot-db-backup.$(hostname).$(whoami).log.INFO.$(date +%Y%m%d-%H%M%S.%N) 2>&1

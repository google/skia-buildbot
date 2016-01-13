#!/bin/bash

WORKDIR=/tmp/buildbot_db_backup
mkdir -p $WORKDIR
pushd $WORKDIR
curl http://localhost:8001/backup > buildbot.db
gsutil cp buildbot.db gs://skia-buildbots/db_backup/$(date +%Y/%m/%d)/buildbot.db
popd
rm -rf $WORKDIR

#!/bin/bash

set -e -x

docker build --no-cache ./dummy --iidfile ./id.txt
ID="$(cat id.txt)"
echo ID: $ID
DEST=${ID#sha256:}
echo $DEST
mkdir $DEST
docker image inspect $ID > $DEST/inspect.json
mkdir $DEST/content
docker-extract $ID /out $DEST/content

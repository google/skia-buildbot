#!/bin/bash
set -e -x

echo $1
pwd
ls $1

echo ${GOOGLE_APPLICATION_CREDENTIALS}
cat ${GOOGLE_APPLICATION_CREDENTIALS}

adb -H 127.0.0.1 -P 10000 push $1 /data/local/tmp/hello
adb -H 127.0.0.1 -P 10000 shell chmod +s /data/local/tmp/hello
adb -H 127.0.0.1 -P 10000 shell /data/local/tmp/hello

#!/bin/bash
set -e -x

adb -H 127.0.0.1 -P 10000 push $1 /data/local/tmp/hello
adb -H 127.0.0.1 -P 10000 shell chmod +s /data/local/tmp/hello
adb -H 127.0.0.1 -P 10000  shell /data/local/tmp/hello

#!/bin/bash
set -e -x

adb push /tmp/hello /data/local/tmp/hello
adb shell chmod +s /data/local/tmp/hello
adb shell /data/local/tmp/hello

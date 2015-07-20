#!/bin/bash

ANDROID_SDK_HOME=""
platformstr=`uname`
if [[ "$platformstr" == 'Linux' ]]; then
  ANDROID_SDK_HOME="$HOME/android-sdk-linux"
elif [[ "$platformstr" == 'Darwin' ]]; then
  ANDROID_SDK_HOME="$HOME/android-sdk-macosx"
fi
$ANDROID_SDK_HOME/tools/android update sdk --no-ui --all --filter build-tools-22.0.1

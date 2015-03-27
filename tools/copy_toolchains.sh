#!/bin/bash

# This script copies Android toolchains into each
# buildslave checkout from the given source.

src="buildbot/skiabot-shuttle-ubuntu12-nexus5-001/build/slave/Perf-Android-Nexus5-Adreno330-Arm7-Release/build/skia/platform_tools/android/toolchains"

function copy_toolchain() {
  dst="$1/build/skia/platform_tools/android"
  if [[ $src == *$dst* ]]; then
    echo "Skipping $dst"
    return
  fi
  if [ -d $dst ]; then
    echo "Copying toolchain to $dst"
    cp -r $src $dst
  fi
}

for d in $(ls buildbot | grep skiabot); do
  prefix="buildbot/$d/build/slave"
  if [ -d $prefix ]; then
    for b in $(ls $prefix | grep "Test"); do
      copy_toolchain $prefix/$b
    done
    for b in $(ls $prefix | grep "Perf"); do
      copy_toolchain $prefix/$b
    done
  fi
done

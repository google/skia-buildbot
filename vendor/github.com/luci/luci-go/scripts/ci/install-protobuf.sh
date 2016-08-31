#!/bin/sh
#
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.
#
# Script for .travis.yaml to install protobuf compiler from source. This is
# needed because APT package for protobuf doesn't support proto3.

set -ex

V="3.0.0-beta-2"
ARCH="x86_64"
DIST="protoc-${V}-linux-${ARCH}"

# Download and unpack the protoc binary distribution package.
wget "https://github.com/google/protobuf/releases/download/v${V}/${DIST}.zip"
unzip "${DIST}.zip"

# Copy protoc into "/usr/bin" and the includes into "/usr/include".
sudo mkdir -p /usr/bin /usr/include
sudo mv "./protoc" /usr/bin/
sudo mv "./google" /usr/include/

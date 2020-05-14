#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Upgrade a bot via apt-get, then reboot. Aborts on error."""


import subprocess

# Copied from
# https://skia.googlesource.com/buildbot/+show/d864d83d992f2968cf4d229cebf2d3104ee11ebf/go/gce/swarming/base-image/setup-script.sh#20
base_cmd = ['sudo', 'DEBIAN_FRONTEND=noninteractive', 'apt',
            '-o', 'quiet=2', '--assume-yes',
            '-o', 'Dpkg::Options::=--force-confdef',
            '-o', 'Dpkg::Options::=--force-confold']

subprocess.check_call(base_cmd + ['update'])
subprocess.check_call(base_cmd + ['full-upgrade'])
subprocess.check_call(base_cmd + ['autoremove'])

subprocess.check_call(['sudo', 'reboot'])

#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Upgrade a bot via apt-get, then reboot. Aborts on error."""


import subprocess

subprocess.check_call(['sudo', 'apt-get', 'update'])
subprocess.check_call(['sudo', 'apt-get', 'dist-upgrade',
                       '-q', '-y', '--ignore-hold'])
subprocess.check_call(['sudo', 'reboot'])

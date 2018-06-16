#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Reboot a Swarming bot."""


import os
import sys


if sys.platform == 'win32':
    os.system('shutdown /r /t 0')
else:
    os.system('sudo reboot')

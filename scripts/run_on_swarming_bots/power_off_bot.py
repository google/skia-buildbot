#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Poweroff a Swarming bot.

Note that this only works on Linux/Mac if the Swarming user is able to run
'sudo shutdown -h now'."""


import os
import time
import sys


if sys.platform == 'win32':
    os.system('shutdown -s')
else:
    os.system('sudo shutdown -h now')

time.sleep(60)

assert false, "Did not shutdown within 60 seconds."

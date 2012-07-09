#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Hooks to run after gclient sync if any files have been updated.
"""

import os
import shutil


def CopyCustomFiles():
  shutil.copyfile(
      'files-to-override/change_macros.html',
      'third_party/chromium_buildbot/third_party/buildbot_8_4p1/buildbot/' +
      'status/web/templates/change_macros.html')
  shutil.copyfile(
      'files-to-override/console.html',
      'third_party/chromium_buildbot/third_party/buildbot_8_4p1/buildbot/' +
      'status/web/templates/console.html')


def Main():
  # cd to the directory where this script lives.
  os.chdir(os.path.dirname(__file__))
  CopyCustomFiles()

if __name__ == '__main__':
  Main()

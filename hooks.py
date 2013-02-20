#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Hooks to run after gclient sync if any files have been updated.
"""

import os


def CopyCustomFiles():
  pass


def Main():
  # cd to the directory where this script lives.
  os.chdir(os.path.dirname(__file__))
  CopyCustomFiles()


if __name__ == '__main__':
  Main()

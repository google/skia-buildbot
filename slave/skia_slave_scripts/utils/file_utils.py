#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities related to file/directory manipulations."""

import os
import shutil


def CreateCleanLocalDir(directory):
  """If directory already exists, it is deleted and recreated."""
  if os.path.exists(directory):
    shutil.rmtree(directory)
  print 'Creating directory: %s' % directory
  os.makedirs(directory)

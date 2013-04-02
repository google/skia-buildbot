#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities related to file/directory manipulations."""

import os
import stat

from common import chromium_utils


def ClearDirectory(directory):
  """ Attempt to clear the contents of a directory. This should only be used
  when the directory itself cannot be removed for some reason. Otherwise,
  chromium_utils.RemoveDirectory or CreateCleanLocalDir should be preferred. """
  for path in os.listdir(directory):
    abs_path = os.path.join(directory, path)
    if os.path.isdir(abs_path):
      chromium_utils.RemoveDirectory(abs_path)
    else:
      if not os.access(abs_path, os.W_OK):
        # Change the path to be writeable
        os.chmod(abs_path, stat.S_IWUSR)
      os.remove(abs_path)


def CreateCleanLocalDir(directory):
  """If directory already exists, it is deleted and recreated."""
  if os.path.exists(directory):
    chromium_utils.RemoveDirectory(directory)
  print 'Creating directory: %s' % directory
  os.makedirs(directory)

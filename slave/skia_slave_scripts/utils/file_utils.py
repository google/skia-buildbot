#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities related to file/directory manipulations."""

import misc
import os
import shutil
import stat


def RecursiveDelete(directory):
  """ Recursively remove a directory tree. Wrapper for shutil.rmtree which
  provides an onerror function in case of permission problems. """

  def _OnRmtreeError(function, path, excinfo):
    """ onerror function for shutil.rmtree.
  
    Reasons we might end up here:
    -  If a file is read-only, rmtree will fail on Windows.
    -  There is a path-length limitation on Windows.  If we exceed that (common),
       then rmtree (and other functions) will fail.
    """
    abs_path = misc.GetAbsPath(path)
    if not os.access(abs_path, os.W_OK):
      # Change the path to be writeable and try again.
      os.chmod(abs_path, stat.S_IWUSR)
    function(abs_path)

  shutil.rmtree(directory, onerror=_OnRmtreeError)


def ClearDirectory(directory):
  """ Attempt to clear the contents of a directory. This should only be used
  when the directory itself cannot be removed for some reason. Otherwise,
  RecursiveDelete or CreateCleanLocalDir should be preferred. """
  for path in os.listdir(path):
    if os.path.isdir(path):
      RecursiveDelete(os.path.join(directory, path))
    else:
      if not os.access(path, os.W_OK):
        # Change the path to be writeable
        os.chmod(path, stat.S_IWUSR)
      os.remove(path)


def CreateCleanLocalDir(directory):
  """If directory already exists, it is deleted and recreated."""
  if os.path.exists(directory):
    RecursiveDelete(directory)
  print 'Creating directory: %s' % directory
  os.makedirs(directory)

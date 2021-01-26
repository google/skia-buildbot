#!/usr/bin/env python
# Copyright (c) 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains miscellaneous tools. """

import os


class ChDir(object):
  """Enter and exit the given directory appropriately."""

  def __init__(self, directory, verbose=True):
    """Instantiate the ChDir.

    Args:
        directory: string; the directory to enter.
        verbose: bool; whether or not to print the directory changes.
    """
    self._destination = directory
    self._origin = None
    self._verbose = verbose

  def __enter__(self):
    """Change to the destination directory.

    Does not check whether the directory exists.
    """
    self._origin = os.getcwd()
    if self._verbose:
      print('chdir %s' % self._destination)
    os.chdir(self._destination)

  def __exit__(self, *args):
    """Change back to the original directory."""
    if self._verbose:
      print('chdir %s' % self._origin)
    os.chdir(self._origin)

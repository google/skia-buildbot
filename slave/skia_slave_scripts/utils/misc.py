#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains miscellaneous tools used by the buildbot scripts. """

import os


# Absolute path to the root of this Skia buildbot checkout.
BUILDBOT_PATH = os.path.realpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    os.pardir, os.pardir, os.pardir))


def ArgsToDict(argv):
  """ Collect command-line arguments of the form '--key value' into a
  dictionary.  Fail if the arguments do not fit this format. """
  dictionary = {}
  PREFIX = '--'
  # Expect the first arg to be the path to the script, which we don't want.
  argv = argv[1:]
  while argv:
    if argv[0].startswith(PREFIX):
      dictionary[argv[0][len(PREFIX):]] = argv[1]
      argv = argv[2:]
    else:
      raise Exception('Malformed input: %s' % argv)
  return dictionary


def ConfirmOptionsSet(name_value_dict):
  """Raise an exception if any of the given command-line options were not set.

  name_value_dict: dictionary mapping option names to option values
  """
  for (name, value) in name_value_dict.iteritems():
    if value is None:
      raise Exception('missing command-line option %s; rerun with --help' %
                      name)


def GetAbsPath(relative_path):
  """My own implementation of os.path.abspath() that better handles paths
  which approach Window's 260-character limit.
  See https://code.google.com/p/skia/issues/detail?id=674

  This implementation adds path components one at a time, resolving the
  absolute path each time, to take advantage of any chdirs into outer
  directories that will shorten the total path length.

  TODO(epoger): share a single implementation with bench_graph_svg.py, instead
  of pasting this same code into both files."""
  if os.path.isabs(relative_path):
    return relative_path
  path_parts = relative_path.split(os.sep)
  abs_path = os.path.abspath('.')
  for path_part in path_parts:
    abs_path = os.path.abspath(os.path.join(abs_path, path_part))
  return abs_path


class ChDir(object):
  """Enter and exit the given directory appropriately."""

  def __init__(self, directory):
    """Instantiate the ChDir.

    Args:
        directory: string; the directory to enter.
    """
    self._destination = directory
    self._origin = None

  def __enter__(self):
    """Change to the destination directory.

    Does not check whether the directory exists.
    """
    self._origin = os.getcwd()
    print 'chdir %s' % self._destination
    os.chdir(self._destination)

  def __exit__(self, *args):
    """Change back to the original directory."""
    print 'chdir %s' % self._origin
    os.chdir(self._origin)


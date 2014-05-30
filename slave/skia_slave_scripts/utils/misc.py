#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains miscellaneous tools used by the buildbot scripts. """

import os

from git_utils import GIT
import shell_utils


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


class GitBranch(object):
  """Class to manage git branches.

  This class allows one to create a new branch in a repository to make changes,
  then it commits the changes, switches to master branch, and deletes the
  created temporary branch upon exit.
  """
  def __init__(self, branch_name, commit_msg, upload=True, commit_queue=False):
    self._branch_name = branch_name
    self._commit_msg = commit_msg
    self._upload = upload
    self._commit_queue = commit_queue
    self._patch_set = 0

  def __enter__(self):
    shell_utils.run([GIT, 'reset', '--hard', 'HEAD'])
    shell_utils.run([GIT, 'checkout', 'master'])
    if self._branch_name in shell_utils.run([GIT, 'branch']):
      shell_utils.run([GIT, 'branch', '-D', self._branch_name])
    shell_utils.run([GIT, 'checkout', '-b', self._branch_name,
                     '-t', 'origin/master'])
    return self

  def commit_and_upload(self, use_commit_queue=False):
    shell_utils.run([GIT, 'commit', '-a', '-m',
                     self._commit_msg])
    upload_cmd = [GIT, 'cl', 'upload', '-f', '--bypass-hooks',
                  '--bypass-watchlists']
    self._patch_set += 1
    if self._patch_set > 1:
      upload_cmd.extend(['-t', 'Patch set %d' % self._patch_set])
    if use_commit_queue:
      upload_cmd.append('--use-commit-queue')
    shell_utils.run(upload_cmd)

  def __exit__(self, exc_type, _value, _traceback):
    if self._upload:
      # Only upload if no error occurred.
      try:
        if exc_type is None:
          self.commit_and_upload(use_commit_queue=self._commit_queue)
      finally:
        shell_utils.run([GIT, 'checkout', 'master'])
        shell_utils.run([GIT, 'branch', '-D', self._branch_name])


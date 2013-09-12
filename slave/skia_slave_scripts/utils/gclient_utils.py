#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities for managing gclient checkouts."""


from common import find_depot_tools

import os
import shell_utils
import time


GIT = 'git.bat' if os.name == 'nt' else 'git'
WHICH = 'where' if os.name == 'nt' else 'which'
SKIA_TRUNK = 'skia'


def _GetGclientPy():
  """ Return the path to the gclient.py file. """
  path_to_gclient = find_depot_tools.add_depot_tools_to_path()
  if path_to_gclient:
    return os.path.join(path_to_gclient, 'gclient.py')
  print 'Falling back on using "gclient" or "gclient.bat"'
  if os.name == 'nt':
    return 'gclient.bat'
  else:
    return 'gclient'


GCLIENT_PY = _GetGclientPy()
GCLIENT_FILE = '.gclient'


def _RunCmd(cmd):
  """ Run a "gclient ..." command. """
  return shell_utils.Bash(['python', GCLIENT_PY] + cmd)


def Config(spec):
  """ Configure a local checkout. """
  return _RunCmd(['config', '--spec=%s' % spec])


def _GetLocalConfig():
  """ Find and return the configuration for the local checkout. """
  if not os.path.isfile(GCLIENT_FILE):
    raise Exception('Unable to find %s' % GCLIENT_FILE)
  config_vars = {}
  exec(open(GCLIENT_FILE).read(), config_vars)
  return config_vars['solutions']


def Sync(revision=None, force=False, delete_unversioned_trees=False,
         branches=None, verbose=False, jobs=None, no_hooks=False):
  """ Update the local checkout to the given revision, if provided, or to the
  most recent revision. """
  cmd = ['sync', '--no-nag-max']
  if verbose:
    cmd.append('--verbose')
  if force:
    cmd.append('--force')
  if delete_unversioned_trees:
    cmd.append('--delete_unversioned_trees')
  if jobs:
    cmd.append('-j%d' % jobs)
  if no_hooks:
    cmd.append('--nohooks')
  if revision and branches and SKIA_TRUNK in branches:
    cmd.extend(['--revision', '%s@%s' % (SKIA_TRUNK, revision)])
  output = _RunCmd(cmd)

  # "gclient sync" just downloads all of the commits. In order to actually sync
  # to the desired commit, we have to "git reset" to that commit.
  start_dir = os.path.abspath(os.curdir)
  if branches and SKIA_TRUNK in branches:
    os.chdir(SKIA_TRUNK)
    if revision:
      shell_utils.Bash([GIT, 'reset', '--hard', revision])
    else:
      shell_utils.Bash([GIT, 'checkout', 'origin/master', '--detach', '-f'])
    os.chdir(start_dir)
  return output


def GetCheckedOutHash():
  """ Determine what commit we actually got. If there are local modifications,
  raise an exception. """
  config = _GetLocalConfig()
  current_directory = os.path.abspath(os.curdir)

  # Get the checked-out commit hash for the first gclient solution.
  os.chdir(config[0]['name'])
  try:
    for _i in xrange(5):
      shell_utils.Bash([WHICH, GIT])
      if os.name != 'nt':
        os.system('alias git')
        os.system('echo $PATH')
      shell_utils.Bash([GIT, '--version'])
      # "git rev-parse HEAD" returns the commit hash for HEAD.
      commit_hash = shell_utils.Bash([GIT, 'rev-parse', 'HEAD']).rstrip('\n')
      # Temporary debugging.
      print 'Got commit_hash: ==%s==' % repr(commit_hash)
      if commit_hash:
        # Break out of the retry loop if we have a non-empty commit hash.
        break
      # Sleep for 20 seconds and hope the next iteration finds the commit hash.
      time.sleep(60)
  finally:
    os.chdir(current_directory)
  return commit_hash


def Revert():
  shell_utils.Bash([GIT, 'clean', '-f', '-d'])
  shell_utils.Bash([GIT, 'reset', '--hard', 'HEAD'])

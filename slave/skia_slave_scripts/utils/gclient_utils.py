#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities for managing gclient checkouts."""


from common import find_depot_tools

import os
import shell_utils


SKIA_TRUNK = 'trunk'


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
         branches=None, verbose=False, manually_grab_svn_rev=False, jobs=None,
         no_hooks=False):
  """ Update the local checkout to the given revision, if provided, or to the
  most recent revision. """
  cmd = ['sync', '--no-nag-max']
  if verbose:
    cmd.append('--verbose')
  if manually_grab_svn_rev:
    cmd.append('--manually_grab_svn_rev')
  if force:
    cmd.append('--force')
  if delete_unversioned_trees:
    cmd.append('--delete_unversioned_trees')
  if jobs:
    cmd.append('-j%d' % jobs)
  if no_hooks:
    cmd.append('--nohooks')
  if revision and SKIA_TRUNK in branches:
    cmd.extend(['--revision', '%s@%s' % (SKIA_TRUNK, revision)])

  return _RunCmd(cmd)


def GetCheckedOutRevision():
  """ Determine what revision we actually got. If there are local modifications,
  raise an exception. """
  config = _GetLocalConfig()
  current_directory = os.path.abspath(os.curdir)
  os.chdir(config[0]['name'])
  if os.name == 'nt':
    svnversion = 'svnversion.bat'
  else:
    svnversion = 'svnversion'
  got_revision = shell_utils.Bash([svnversion, '.'], echo=False)
  os.chdir(current_directory)
  try:
    return int(got_revision)
  except ValueError:
    raise Exception('Working copy is dirty! Got revision: %s' % got_revision)


def Revert():
  """ Revert all local changes. """
  _RunCmd(['revert', '-j1'])


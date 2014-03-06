#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities for managing gclient checkouts."""


from common import find_depot_tools

import os
import shell_utils


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
  return shell_utils.run(['python', GCLIENT_PY] + cmd)


def Config(spec):
  """ Configure a local checkout. """
  return _RunCmd(['config', '--spec=%s' % spec])


def _GetLocalConfig():
  """Find and return the configuration for the local checkout.

  Returns: tuple of the form (checkout_root, solutions_dict), where
      checkout_root is the path to the directory containing the .glient file,
      and solutions_dict is the dictionary of solutions defined in .gclient.
  """
  checkout_root = os.path.abspath(os.curdir)
  depth = len(checkout_root.split(os.path.sep))
  # Start with the current working directory and move upwards until we find the
  # .gclient file.
  while not os.path.isfile(os.path.join(checkout_root, GCLIENT_FILE)):
    if not depth:
      raise Exception('Unable to find %s' % GCLIENT_FILE)
    checkout_root = os.path.abspath(os.path.join(checkout_root, os.pardir))
    depth -= 1
  config_vars = {}
  exec(open(os.path.join(checkout_root, GCLIENT_FILE)).read(), config_vars)
  return checkout_root, config_vars['solutions']


def maybe_fix_identity(username='chrome-bot', email='skia.committer@gmail.com'):
  """If either of user.name or user.email is not defined, define it."""
  try:
    shell_utils.run([GIT, 'config', '--get', 'user.name'])
  except shell_utils.CommandFailedException:
    shell_utils.run([GIT, 'config', 'user.name', '"%s"' % username])

  try:
    shell_utils.run([GIT, 'config', '--get', 'user.email'])
  except shell_utils.CommandFailedException:
    shell_utils.run([GIT, 'config', 'user.email', '"%s"' % email])


def Sync(revisions=None, force=False, delete_unversioned_trees=False,
         verbose=False, jobs=None, no_hooks=False, extra_args=None):
  """ Update the local checkout using gclient.

  Args:
      revisions: optional list of (branch, revision) tuples indicating which
          projects to sync to which revisions.
      force: whether to run with --force.
      delete_unversioned_trees: whether to run with --delete-unversioned-trees.
      verbose: whether to run with --verbose.
      jobs: optional argument for the --jobs flag.
      no_hooks: whether to run with --nohooks.
      extra_args: optional list; any additional arguments.
  """

  start_dir = os.path.abspath(os.curdir)
  for branch, _ in (revisions or []):
    # Do whatever it takes to get up-to-date with origin/master.
    if os.path.exists(branch):
      os.chdir(branch)
      # First, fix the git identity if needed.
      maybe_fix_identity()

      # If there are local changes, "git checkout" will fail.
      shell_utils.run([GIT, 'reset', '--hard', 'HEAD'])
      # In case HEAD is detached...
      shell_utils.run([GIT, 'checkout', 'master'])
      # This updates us to origin/master even if master has diverged.
      shell_utils.run([GIT, 'reset', '--hard', 'origin/master'])
      os.chdir(start_dir)

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
  for branch, revision in (revisions or []):
    if revision:
      cmd.extend(['--revision', '%s@%s' % (branch, revision)])
  if extra_args:
    cmd.extend(extra_args)
  output = _RunCmd(cmd)

  # "gclient sync" just downloads all of the commits. In order to actually sync
  # to the desired commit, we have to "git reset" to that commit.
  for branch, revision in (revisions or []):
    os.chdir(branch)
    if revision:
      shell_utils.run([GIT, 'reset', '--hard', revision])
    else:
      shell_utils.run([GIT, 'reset', '--hard', 'origin/master'])
    os.chdir(start_dir)
  return output


def GetCheckedOutHash():
  """ Determine what commit we actually got. If there are local modifications,
  raise an exception. """
  checkout_root, config_dict = _GetLocalConfig()
  current_directory = os.path.abspath(os.curdir)

  # Get the checked-out commit hash for the first gclient solution.
  os.chdir(os.path.join(checkout_root, config_dict[0]['name']))
  try:
    # First, print out the remote from which we synced, just for debugging.
    cmd = [GIT, 'remote', '-v']
    try:
      shell_utils.run(cmd)
    except shell_utils.CommandFailedException as e:
      print e

    # "git rev-parse HEAD" returns the commit hash for HEAD.
    return shell_utils.run([GIT, 'rev-parse', 'HEAD'],
                           log_in_real_time=False).rstrip('\n')
  finally:
    os.chdir(current_directory)


def Revert():
  shell_utils.run([GIT, 'clean', '-f', '-d'])
  shell_utils.run([GIT, 'reset', '--hard', 'HEAD'])

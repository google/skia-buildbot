#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities for managing gclient checkouts."""


import os

import misc
import shell_utils


SKIA_TRUNK = 'skia'
GIT = 'git.bat' if os.name == 'nt' else 'git'
GCLIENT_PY = 'gclient.bat' if os.name == 'nt' else 'gclient'
GCLIENT_FILE = '.gclient'


def _RunCmd(cmd):
  """ Run a "gclient ..." command with retries. """
  return shell_utils.run_retry([GCLIENT_PY] + cmd, attempts=3)


def GClient():
  """Run "gclient" without any command.

  This is useful because gclient installs things and updates itself, and we may
  want it to do so before we attempt to do other things.
  """
  return _RunCmd([])


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
  with open(os.path.join(checkout_root, GCLIENT_FILE), 'rb') as f:
    exec(f.read(), config_vars)
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
  for branch, _ in (revisions or []):
    # Do whatever it takes to get up-to-date with origin/master.
    if os.path.exists(branch):
      with misc.ChDir(branch):
        # First, fix the git identity if needed.
        maybe_fix_identity()

        # If there are local changes, "git checkout" will fail.
        shell_utils.run([GIT, 'reset', '--hard', 'HEAD'])
        # In case HEAD is detached...
        shell_utils.run([GIT, 'checkout', 'master'])
        # Always fetch, in case we're unmanaged.
        shell_utils.run_retry([GIT, 'fetch'], attempts=5)
        # This updates us to origin/master even if master has diverged.
        shell_utils.run([GIT, 'reset', '--hard', 'origin/master'])

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
    with misc.ChDir(branch):
      if revision:
        shell_utils.run([GIT, 'reset', '--hard', revision])
      else:
        shell_utils.run([GIT, 'reset', '--hard', 'origin/master'])
  return output


def GetCheckedOutHash():
  """ Determine what commit we actually got. If there are local modifications,
  raise an exception. """
  checkout_root, config_dict = _GetLocalConfig()

  # Get the checked-out commit hash for the first gclient solution.
  with misc.ChDir(os.path.join(checkout_root, config_dict[0]['name'])):
    # First, print out the remote from which we synced, just for debugging.
    cmd = [GIT, 'remote', '-v']
    try:
      shell_utils.run(cmd)
    except shell_utils.CommandFailedException as e:
      print e

    # "git rev-parse HEAD" returns the commit hash for HEAD.
    return shell_utils.run([GIT, 'rev-parse', 'HEAD'],
                           log_in_real_time=False).rstrip('\n')


def GetGitRepoPOSIXTimestamp():
  """Returns the POSIX timestamp for the current Skia commit as in int."""
  git_show_command = [GIT, 'show', '--format=%at', '-s']
  raw_timestamp = shell_utils.run(
      git_show_command, log_in_real_time=False, echo=False,
      print_timestamps=False)
  return int(raw_timestamp)


# than extract the number for the current repo
def GetGitNumber(commit_hash):
  """Returns the GIT number for the current Skia commit as in int."""
  try:
    git_show_command = [GIT, 'number']
    git_number = shell_utils.run(
        git_show_command, log_in_real_time=False, echo=False,
        print_timestamps=False)
    return int(git_number)
  except shell_utils.CommandFailedException:
    print 'GetGitNumber: Unable to get git number, returning -1'
    return -1


def Revert():
  shell_utils.run([GIT, 'clean', '-f', '-d'])
  shell_utils.run([GIT, 'reset', '--hard', 'HEAD'])


def RunHooks(gyp_defines=None, gyp_generators=None):
  """ Run "gclient runhooks".

  Args:
      gyp_defines: optional string; GYP_DEFINES to be passed to Gyp.
      gyp_generators: optional string; which GYP_GENERATORS to use.
  """
  if gyp_defines:
    os.environ['GYP_DEFINES'] = gyp_defines
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
  if gyp_generators:
    os.environ['GYP_GENERATORS'] = gyp_generators
    print 'GYP_GENERATORS="%s"' % os.environ['GYP_GENERATORS']

  _RunCmd(['runhooks'])

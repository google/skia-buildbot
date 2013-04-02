#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """

from utils import file_utils
from utils import shell_utils
from build_step import BuildStep, BuildStepFailure
from config_private import SKIA_SVN_BASEURL
import ast
import os
import sys
import time


def DeleteCheckoutAndGetCleanOne(gclient, gclient_spec, sync_args):
  """ Delete the entire checkout and create a new one.

  gclient: string; the name of the gclient script.
  gclient_spec: string; gclient's specification for which directories to check
      out.
  sync_args: list of strings; extra arguments to pass to 'gclient sync'.
  """
  build_dir = os.path.abspath(os.curdir)
  os.chdir(os.pardir)
  print 'Deleting checkout and starting over...'
  file_utils.ClearDirectory(build_dir)
  os.chdir(build_dir)
  shell_utils.Bash([gclient, 'config', '--spec=%s' % gclient_spec])
  shell_utils.Bash([gclient, 'sync'] + sync_args)


def GetCheckedOutRevision(trunk_path):
  """ Determine what revision we actually got. If there are local modifications,
  raise an exception.

  trunk_path: string, path to the directory where revision number will be
      queried.
  """
  current_directory = os.path.abspath(os.curdir)
  os.chdir(trunk_path)
  if os.name == 'nt':
    svnversion = 'svnversion.bat'
  else:
    svnversion = 'svnversion'
  got_revision = shell_utils.Bash([svnversion, '.'], echo=False)
  os.chdir(current_directory)
  try:
    return int(got_revision)
  except ValueError:
    raise Exception('Working copy is dirty!')


def RevertLocalChanges(gclient):
  """ Attempt to restore the checkout to a "clean" state.

  gclient: string; the name of the gclient script.
  """
  # "gclient revert" is finicky on Windows (for more information, see
  # https://code.google.com/p/skia/issues/detail?id=1041).  The number of
  # retries required can be arbitrarily large (limited by the number of
  # directories in the entire checkout), so we just retry until either:
  #
  # - The retry succeeds
  # - The timeout is reached
  # - The same error is encountered twice in a row
  #
  # The logic behind the last case is that, if the error is irrecoverable,
  # we will see the same output multiple times, whereas if the error is
  # recoverable, we won't see the same output again when we wait and
  # retry.
  last_error_text = None
  attempt = 1
  while True:
    print 'Reverting local changes...'
    proc = shell_utils.BashAsync([gclient, 'revert', '-j1'])
    returncode, output = shell_utils.LogProcessToCompletion(proc)
    if returncode == 0:
      break
    print 'Revert failed attempt #' + str(attempt)
    if output == last_error_text:
      # Assume that we've hit an irrecoverable error if we see the same
      # error more than once in a row
      raise Exception('Revert failed with the same output twice in a row.'
                      ' Not attempting further reverts.')
    last_error_text = output
    attempt = attempt + 1
    # Sleep before retrying revert.
    time.sleep(1)

class Update(BuildStep):
  def __init__(self, timeout=6000, no_output_timeout=4800, **kwargs):
    super(Update, self).__init__(timeout=timeout,
                                 no_output_timeout=no_output_timeout, **kwargs)

  def _Run(self):
    if os.name == 'nt':
      gclient = 'gclient.bat'
      which = 'where'
      svn = 'svn.bat'
    else:
      gclient = 'gclient'
      which = 'which'
      svn = 'svn'

    # Print out the location of the depot_tools.
    shell_utils.Bash([which, gclient])

    # Sometimes the build slaves "forget" the svn server. To prevent this from
    # occurring, use "svn ls" with --trust-server-cert.
    shell_utils.Bash([svn, 'ls', SKIA_SVN_BASEURL,
                      '--non-interactive', '--trust-server-cert'], echo=False)

    # We receive gclient_solutions as a list of dictionaries flattened into a
    # double-quoted string. This invocation of literal_eval converts that string
    # into a list of strings.
    solutions = ast.literal_eval(self._args['gclient_solutions'][1:-1])

    # Parse each solution dictionary from a string and add it to a list, while
    # building a string to pass as a spec to gclient, to indicate which
    # branches should be downloaded.
    solution_dicts = []
    gclient_spec = 'solutions = ['
    for solution in solutions:
      gclient_spec += solution
      solution_dicts += ast.literal_eval(solution)
    gclient_spec += ']'

    # Run "gclient config" with the spec we just built.
    shell_utils.Bash([gclient, 'config', '--spec=%s' % gclient_spec])

    # Construct an argument list for "gclient sync".
    sync_args = ['--verbose', '--manually_grab_svn_rev', '--force',
                 '--delete_unversioned_trees']
    if self._revision:
      # If we're syncing to a specific revision, we have to specify that
      # revision for each branch.
      for solution in solution_dicts:
        sync_args += ['--revision', '%s@%d' % (solution['name'],
                                               self._revision)]

    try:
      if self._is_try:
        # Clean our checkout to make sure we don't have a patch left over.
        RevertLocalChanges(gclient)

      # Run "gclient sync" with the argument list we just constructed.
      shell_utils.Bash([gclient, 'sync'] + sync_args)

      got_revision = GetCheckedOutRevision(solution_dicts[0]['name'])

    except Exception as e:
      print e
      # If the sync failed, remove the entire build directory and start over.
      DeleteCheckoutAndGetCleanOne(gclient, gclient_spec, sync_args)
      got_revision = GetCheckedOutRevision(solution_dicts[0]['name'])

    # If the revision we actually got differs from what was requested, raise an
    # exception.
    if self._revision and got_revision != self._revision:
      raise BuildStepFailure('Actually-synced revision is different from the '
                             'requested revision.')

    # Print the obtained revision number so that the master can parse it.
    print 'Skia updated to revision %d' % got_revision


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Update))

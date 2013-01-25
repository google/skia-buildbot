#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """

from utils import file_utils
from utils import shell_utils
from build_step import BuildStep, BuildStepFailure
import ast
import config
import os
import sys
import time


class Update(BuildStep):
  def __init__(self, timeout=6000, no_output_timeout=4800, **kwargs):
    super(Update, self).__init__(timeout=timeout,
                                 no_output_timeout=no_output_timeout, **kwargs)

  def _Run(self):
    if os.name == 'nt':
      gclient = 'gclient.bat'
      svn = 'svn.bat'
    else:
      gclient = 'gclient'
      svn = 'svn'

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

    if self._is_try:
      # Clean our checkout to make sure we don't have a patch left over.

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
      # recoverable, we won't see the same output again when we wait and retry.
      last_error_text = None
      attempt = 1
      while True:
        print 'Cleaning checkout...'
        proc = shell_utils.BashAsync([gclient, 'revert', '-j1'])
        returncode, output = shell_utils.LogProcessToCompletion(proc)
        if returncode == 0:
          break
        print 'Revert failed attempt #' + str(attempt)
        if output == last_error_text:
          # Assume that we've hit an irrecoverable error if we see the same
          # error more than once in a row
          raise Exception('Revert failed with the same output twice in a row. '
                          'Interpreting this error as irrecoverable and giving '
                          'up.')
        last_error_text = output
        attempt = attempt + 1
        # Sleep before retrying revert.
        time.sleep(1)

    # Sometimes the build slaves "forget" the svn server. To prevent this from
    # occurring, use "svn ls" with --trust-server-cert.
    shell_utils.Bash([svn, 'ls', config.Master.skia_url,
                      '--non-interactive', '--trust-server-cert'], echo=False)

    try:
      # Run "gclient sync" with the argument list we just constructed.
      shell_utils.Bash([gclient, 'sync'] + sync_args)
    except Exception:
      # If the sync failed, remove the entire build directory and start over.
      build_dir = os.path.abspath(os.curdir)
      os.chdir(os.pardir)
      print 'Deleting checkout and starting over...'
      file_utils.ClearDirectory(build_dir)
      os.chdir(build_dir)
      shell_utils.Bash([gclient, 'config', '--spec=%s' % gclient_spec])
      shell_utils.Bash([gclient, 'sync'] + sync_args)

    # Determine what revision we actually got. If it differs from what was
    # requested, this step fails.
    os.chdir(solution_dicts[0]['name'])
    try:
      if os.name == 'nt':
        svnversion = 'svnversion.bat'
      else:
        svnversion = 'svnversion'
      got_revision = int(shell_utils.Bash([svnversion, '.'], echo=False))
    except:
      raise BuildStepFailure('Working copy is dirty!')

    if self._revision and got_revision != self._revision:
      raise BuildStepFailure('Actually-synced revision is different from the '
                             'requested revision.')

    # Print the obtained revision number so that the master can parse it.
    print 'Skia updated to revision %d' % got_revision


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Update))

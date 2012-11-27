#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """

from utils import shell_utils
from build_step import BuildStep, BuildStepFailure
import ast
import config
import os
import re
import shutil
import sys


class Update(BuildStep):
  def _Run(self):
    if os.name == 'nt':
      gclient = 'gclient.bat'
    else:
      gclient = 'gclient'

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

    # Sometimes the build slaves "forget" the svn server. To prevent this from
    # occurring, use "svn ls" with --trust-server-cert.
    shell_utils.Bash(['svn', 'ls', config.Master.skia_url,
                      '--non-interactive', '--trust-server-cert'], echo=False)

    # Run "gclient sync" with the argument list we just constructed.
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
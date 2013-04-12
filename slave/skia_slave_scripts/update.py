#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from utils import gclient_utils
from utils import shell_utils
from build_step import BuildStep, BuildStepFailure
from config_private import SKIA_SVN_BASEURL
import ast
import os
import sys


class Update(BuildStep):
  def __init__(self, timeout=6000, no_output_timeout=4800, attempts=5,
               **kwargs):
    super(Update, self).__init__(timeout=timeout,
                                 no_output_timeout=no_output_timeout,
                                 attempts=attempts,
                                 **kwargs)

  def _Run(self):
    if os.name == 'nt':
      svn = 'svn.bat'
    else:
      svn = 'svn'

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
    gclient_utils.Config(spec=gclient_spec)

    if self._is_try:
      # Clean our checkout to make sure we don't have a patch left over.
      gclient_utils.Revert()

    # Run "gclient sync"
    gclient_utils.Sync(
        branches=[solution['name'] for solution in solution_dicts],
        revision=self._revision,
        verbose=True,
        manually_grab_svn_rev=True,
        force=True,
        delete_unversioned_trees=True)

    got_revision = gclient_utils.GetCheckedOutRevision()

    # If the revision we actually got differs from what was requested, raise an
    # exception.
    if self._revision and got_revision != self._revision:
      raise BuildStepFailure('Actually-synced revision is different from the '
                             'requested revision.')

    # Print the obtained revision number so that the master can parse it.
    print 'Skia updated to revision %d' % got_revision


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Update))

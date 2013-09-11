#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from utils import file_utils
from utils import gclient_utils
from build_step import BuildStep, BuildStepFailure
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

    # Set the DEPS target_os if necessary.
    if self._deps_target_os:
      gclient_spec += '\ntarget_os = ["%s"]' % self._deps_target_os

    # Run "gclient config" with the spec we just built.
    gclient_utils.Config(spec=gclient_spec)

    # Run "gclient sync"
    try:
      if self._is_try:
        # Clean our checkout to make sure we don't have a patch left over.
        gclient_utils.Revert()
      gclient_utils.Sync(
          branches=[solution['name'] for solution in solution_dicts],
          revision=self._revision,
          verbose=True,
          force=True,
          delete_unversioned_trees=True)
      got_revision = gclient_utils.GetCheckedOutHash()
    except Exception:
      # If the sync fails, clear the checkout and try again.
      build_dir = os.path.abspath(os.curdir)
      os.chdir(os.pardir)
      file_utils.ClearDirectory(build_dir)
      os.chdir(build_dir)
      gclient_utils.Config(spec=gclient_spec)
      gclient_utils.Sync(
          branches=[solution['name'] for solution in solution_dicts],
          revision=self._revision,
          verbose=True,
          force=True,
          delete_unversioned_trees=True,
          jobs=1)
      got_revision = gclient_utils.GetCheckedOutHash()

    # If the revision we actually got differs from what was requested, raise an
    # exception.
    if self._revision and got_revision != self._revision:
      raise BuildStepFailure('Actually-synced revision "%s" is different from '
                             'the requested revision "%s".' % (
                                  repr(got_revision), repr(self._revision)))

    # Print the obtained revision number so that the master can parse it.
    print 'Skia updated to %s' % got_revision


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Update))

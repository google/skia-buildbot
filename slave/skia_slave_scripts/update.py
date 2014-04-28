#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from common import chromium_utils
from utils import file_utils
from utils import gclient_utils
from utils import misc
from utils import shell_utils
from build_step import BuildStep, BuildStepFailure

import ast
import config_private
import os
import re
import sys


LOCAL_GIT_MIRROR_URL = 'http://192.168.1.120/git-mirror/skia'
SKIA_GIT_URL_TO_REPLACE = config_private.SKIA_GIT_URL[:-len('.git')]


def _MaybeUseSkiaLabMirror(revision=None):
  """If the SkiaLab mirror is reachable, set the gitconfig to use that instead
  of the remote repo.

  Args:
      revision: optional string; commit hash to which we're syncing. This is a
          safety net; in the case that the mirror does not yet have this commit,
          we will use the remote repo instead.
  """
  # Attempt to reach the SkiaLab git mirror.
  mirror_is_accessible = False
  print 'Attempting to reach the SkiaLab git mirror...'
  try:
    shell_utils.run([gclient_utils.GIT, 'ls-remote',
                     LOCAL_GIT_MIRROR_URL + '.git',
                     revision or 'HEAD', '--exit-code'], timeout=10)
    mirror_is_accessible = True
  except (shell_utils.CommandFailedException, shell_utils.TimeoutException):
    pass

  # Find the global git config entries and loop over them, removing the ones
  # which aren't needed and adding a URL override for the git mirror if it is
  # accessible and not already present.
  try:
    configs = shell_utils.run([gclient_utils.GIT, 'config', '--global',
                               '--list']).splitlines()
  except shell_utils.CommandFailedException:
    configs = []

  already_overriding_url = False
  for config in configs:
    override_url = None
    match = re.match('url.(.+).insteadof=', config)
    if match:
      override_url = match.groups()[0]
    if override_url:
      if override_url == LOCAL_GIT_MIRROR_URL and mirror_is_accessible:
        print 'Already have URL override for SkiaLab git mirror.'
        already_overriding_url = True
      else:
        print 'Removing unneeded URL override for %s' % override_url
        try:
          shell_utils.run([gclient_utils.GIT, 'config', '--global',
                           '--remove-section', config.split('.insteadof')[0]])
        except shell_utils.CommandFailedException as e:
          if 'No such section!' in e.output:
            print '"insteadof" section already removed; continuing...'
          else:
            raise

  if mirror_is_accessible and not already_overriding_url:
    print ('SkiaLab git mirror appears to be accessible. Changing gitconfig to '
           'use the mirror.')
    shell_utils.run([gclient_utils.GIT, 'config', '--global',
                     'url.%s.insteadOf' % LOCAL_GIT_MIRROR_URL,
                     SKIA_GIT_URL_TO_REPLACE])

  # Some debugging info that might help us figure things out...
  try:
    shell_utils.run([gclient_utils.GIT, 'config', '--global', '--list'])
  except shell_utils.CommandFailedException:
    pass


class Update(BuildStep):
  def __init__(self, timeout=10000, no_output_timeout=6000, attempts=5,
               **kwargs):
    super(Update, self).__init__(timeout=timeout,
                                 no_output_timeout=no_output_timeout,
                                 attempts=attempts,
                                 **kwargs)

  def _Run(self):
    _MaybeUseSkiaLabMirror(self._revision)

    # We receive gclient_solutions as a list of dictionaries flattened into a
    # double-quoted string. This invocation of literal_eval converts that string
    # into a list of strings.
    solutions = ast.literal_eval(self._args['gclient_solutions'][1:-1])

    # TODO(borenet): Move the gclient solutions parsing logic into a function.

    # Parse each solution dictionary from a string and add it to a list, while
    # building a string to pass as a spec to gclient, to indicate which
    # branches should be downloaded.
    solution_dicts = []
    gclient_spec = 'solutions = ['
    for solution in solutions:
      gclient_spec += solution
      solution_dicts += ast.literal_eval(solution)
    gclient_spec += ']'

    # Use a cache-dir.
    gclient_spec += '\ncache_dir = \'%s\'' % gclient_utils.DEFAULT_GCLIENT_CACHE

    # Set the DEPS target_os if necessary.
    if self._deps_target_os:
      gclient_spec += '\ntarget_os = ["%s"]' % self._deps_target_os

    # Run "gclient config" with the spec we just built.
    gclient_utils.Config(spec=gclient_spec)

    revisions = []
    for solution in solution_dicts:
      if solution['name'] == gclient_utils.SKIA_TRUNK:
        revisions.append((solution['name'], self._revision))
      else:
        url_split = solution['url'].split('@')
        if len(url_split) > 1:
          revision = url_split[1]
          revisions.append((solution['name'], revision))

    try:
      if self._is_try:
        # Clean our checkout to make sure we don't have a patch left over.
        if (os.path.isdir('skia') and
            os.path.isdir(os.path.join('skia', '.git'))):
          with misc.ChDir('skia'):
            gclient_utils.Revert()

      # Run "gclient sync"
      gclient_utils.Sync(
          revisions=revisions,
          verbose=True,
          force=True,
          delete_unversioned_trees=True)
      got_revision = gclient_utils.GetCheckedOutHash()
    except Exception:
      # If the sync fails, clear the checkout and try again.
      print 'Initial sync failed.'
      # Attempt to remove the skia directory first.
      if os.path.isdir('skia'):
        print 'Removing "skia"'
        chromium_utils.RemoveDirectory('skia')
      # Now, remove *everything* in the build directory.
      build_dir = os.path.abspath(os.curdir)
      with misc.ChDir(os.pardir):
        print 'chdir %s' % os.getcwd()
        print 'Attempting to clear %s' % build_dir
        file_utils.clear_directory(build_dir)
      print 'chdir %s' % os.getcwd()
      # Try to sync again.
      print 'Attempting to sync again.'
      gclient_utils.Config(spec=gclient_spec)
      gclient_utils.Sync(
          revisions=revisions,
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

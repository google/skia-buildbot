#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from common import chromium_utils
from utils import gclient_utils, shell_utils
from build_step import BuildStep, BuildStepFailure
import ast
import os
import socket
import sys


# TODO(epoger): temporarily added to use the git mirror behind our NAT
def _PopulateGitConfigFile(builder_name):
  print 'entering _PopulateGitConfigFile()...'

  s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
  s.connect(('google.com', 80))
  my_ipaddr = s.getsockname()[0]
  print '  I think my IP address is "%s"' % my_ipaddr

  git_path = shell_utils.Bash([gclient_utils.WHICH, gclient_utils.GIT])
  if 'depot_tools' in git_path:
    destpath = os.path.join(os.path.dirname(git_path), '.gitconfig')
  else:
    destpath = os.path.join(os.path.expanduser('~'), '.gitconfig')
  destfile = open(destpath, 'w')
  if my_ipaddr.startswith('192.168.1.'):
    print ('  I think I am behind the NAT router. '
           'Writing to .gitconfig file at "%s"...' % destpath)
    git_config = ('[url "http://192.168.1.122/git-mirror/skia"]\n'
                  '   insteadOf = https://skia.googlesource.com/skia\n')
    if 'Mac10.6' in builder_name:
      git_config += '[http]\n   postBuffer = 524288000\n'
    destfile.write(git_config)
  else:
    print ('  I think I am NOT behind the NAT router. '
           'Writing a blank .gitconfig file to "%s".' % destpath)
    destfile.write('')
  destfile.close()

  # Some debugging info that might help us figure things out...
  cmd = [gclient_utils.GIT, 'config', '--global', '--list']
  try:
    shell_utils.Bash(cmd)
  except Exception as e:
    print '  caught exception %s while trying to run command %s' % (e, cmd)
  print 'leaving _PopulateGitConfigFile()'


class Update(BuildStep):
  def __init__(self, timeout=10000, no_output_timeout=6000, attempts=5,
               **kwargs):
    super(Update, self).__init__(timeout=timeout,
                                 no_output_timeout=no_output_timeout,
                                 attempts=attempts,
                                 **kwargs)

  def _Run(self):
    # If an old SVN checkout of Skia exists, remove it.
    if os.path.isdir('trunk'):
      print 'Removing old Skia checkout at %s' % os.path.abspath('trunk')
      chromium_utils.RemoveDirectory('trunk')

    _PopulateGitConfigFile(self._builder_name)

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

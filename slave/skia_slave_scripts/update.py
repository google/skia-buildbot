#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from utils import file_utils
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

    ############################################################################
    # In preparation for the switch to git, pre-sync the git repository on some
    # of the bots.
    # TODO(borenet): Remove this once the git switch is finished.
    presync_bots = [
      #'Build-Mac10.6-GCC-x86-Debug',
      #'Build-Mac10.6-GCC-x86-Release',
      #'Build-Mac10.6-GCC-x86_64-Debug',
      #'Build-Mac10.6-GCC-x86_64-Release',
      #'Build-Mac10.7-Clang-Arm7-Debug-iOS',
      #'Build-Mac10.7-Clang-Arm7-Release-iOS',
      #'Build-Mac10.7-Clang-x86-Debug',
      #'Build-Mac10.7-Clang-x86-Release',
      #'Build-Mac10.7-Clang-x86_64-Debug',
      #'Build-Mac10.7-Clang-x86_64-Release',
      #'Build-Mac10.8-Clang-x86-Debug',
      #'Build-Mac10.8-Clang-x86-Release',
      #'Build-Mac10.8-Clang-x86_64-Debug',
      #'Build-Mac10.8-Clang-x86_64-Release',
      #'Build-Ubuntu12-Clang-x86_64-Debug',
      #'Build-Ubuntu12-GCC-Arm7-Debug-Daisy',
      #'Build-Ubuntu12-GCC-Arm7-Debug-GalaxyNexus',
      #'Build-Ubuntu12-GCC-Arm7-Debug-Nexus10',
      #'Build-Ubuntu12-GCC-Arm7-Debug-Nexus4',
      #'Build-Ubuntu12-GCC-Arm7-Debug-Nexus7',
      #'Build-Ubuntu12-GCC-Arm7-Debug-NexusS',
      #'Build-Ubuntu12-GCC-Arm7-Debug-Xoom',
      #'Build-Ubuntu12-GCC-Arm7-Release-Daisy',
      #'Build-Ubuntu12-GCC-Arm7-Release-GalaxyNexus',
      #'Build-Ubuntu12-GCC-Arm7-Release-Nexus10',
      #'Build-Ubuntu12-GCC-Arm7-Release-Nexus4',
      #'Build-Ubuntu12-GCC-Arm7-Release-Nexus7',
      #'Build-Ubuntu12-GCC-Arm7-Release-NexusS',
      #'Build-Ubuntu12-GCC-Arm7-Release-Xoom',
      #'Build-Ubuntu12-GCC-NaCl-Debug',
      #'Build-Ubuntu12-GCC-NaCl-Release',
      #'Build-Ubuntu12-GCC-x86-Debug',
      #'Build-Ubuntu12-GCC-x86-Debug-Alex',
      #'Build-Ubuntu12-GCC-x86-Debug-IntelRhb',
      #'Build-Ubuntu12-GCC-x86-Release',
      #'Build-Ubuntu12-GCC-x86-Release-Alex',
      #'Build-Ubuntu12-GCC-x86-Release-IntelRhb',
      #'Build-Ubuntu12-GCC-x86_64-Debug',
      #'Build-Ubuntu12-GCC-x86_64-Debug-Link',
      #'Build-Ubuntu12-GCC-x86_64-Debug-NoGPU',
      #'Build-Ubuntu12-GCC-x86_64-Release',
      #'Build-Ubuntu12-GCC-x86_64-Release-Link',
      #'Build-Ubuntu12-GCC-x86_64-Release-NoGPU',
      #'Build-Ubuntu12-GCC-x86_64-Release-Valgrind',
      #'Build-Ubuntu13-Clang-x86_64-Debug-ASAN',
      #'Build-Ubuntu13-GCC4.8-x86_64-Debug',
      #'Build-Win7-VS2010-x86-Debug',
      #'Build-Win7-VS2010-x86-Debug-ANGLE',
      #'Build-Win7-VS2010-x86-Debug-DirectWrite',
      #'Build-Win7-VS2010-x86-Debug-Exceptions',
      #'Build-Win7-VS2010-x86-Release',
      #'Build-Win7-VS2010-x86-Release-ANGLE',
      #'Build-Win7-VS2010-x86-Release-DirectWrite',
      #'Build-Win7-VS2010-x86_64-Debug',
      #'Build-Win7-VS2010-x86_64-Release',
      #'Build-Win8-VS2012-x86-Debug',
      #'Build-Win8-VS2012-x86-Release',
      #'Build-Win8-VS2012-x86_64-Debug',
      #'Build-Win8-VS2012-x86_64-Release',
      #'Canary-Chrome-Ubuntu12-Ninja-x86_64-Default',
      #'Canary-Chrome-Win7-Ninja-x86-SharedLib',
      #'Canary-Moz2D-Ubuntu12-GCC-x86_64-Release',
      #'Housekeeper-Nightly',
      #'Housekeeper-PerCommit',
      #'Perf-Android-GalaxyNexus-SGX540-Arm7-Release',
      #'Perf-Android-IntelRhb-SGX544-x86-Release',
      #'Perf-Android-Nexus10-MaliT604-Arm7-Release',
      #'Perf-Android-Nexus4-Adreno320-Arm7-Release',
      #'Perf-Android-Nexus7-Tegra3-Arm7-Release',
      #'Perf-Android-NexusS-SGX540-Arm7-Release',
      #'Perf-Android-Xoom-Tegra2-Arm7-Release',
      #'Perf-ChromeOS-Alex-GMA3150-x86-Release',
      #'Perf-ChromeOS-Daisy-MaliT604-Arm7-Release',
      #'Perf-ChromeOS-Link-HD4000-x86_64-Release',
      #'Perf-Mac10.6-MacMini4.1-GeForce320M-x86-Release',
      #'Perf-Mac10.6-MacMini4.1-GeForce320M-x86_64-Release',
      #'Perf-Mac10.7-MacMini4.1-GeForce320M-x86-Release',
      #'Perf-Mac10.7-MacMini4.1-GeForce320M-x86_64-Release',
      #'Perf-Mac10.8-MacMini4.1-GeForce320M-x86-Release',
      #'Perf-Mac10.8-MacMini4.1-GeForce320M-x86_64-Release',
      #'Perf-Ubuntu12-ShuttleA-ATI5770-x86-Release',
      #'Perf-Ubuntu12-ShuttleA-ATI5770-x86_64-Release',
      #'Perf-Win7-ShuttleA-HD2000-x86-Release',
      #'Perf-Win7-ShuttleA-HD2000-x86-Release-ANGLE',
      #'Perf-Win7-ShuttleA-HD2000-x86-Release-DirectWrite',
      #'Perf-Win7-ShuttleA-HD2000-x86_64-Release',
      #'Test-Android-GalaxyNexus-SGX540-Arm7-Debug',
      #'Test-Android-GalaxyNexus-SGX540-Arm7-Release',
      #'Test-Android-IntelRhb-SGX544-x86-Debug',
      #'Test-Android-IntelRhb-SGX544-x86-Release',
      #'Test-Android-Nexus10-MaliT604-Arm7-Debug',
      #'Test-Android-Nexus10-MaliT604-Arm7-Release',
      #'Test-Android-Nexus4-Adreno320-Arm7-Debug',
      #'Test-Android-Nexus4-Adreno320-Arm7-Release',
      #'Test-Android-Nexus7-Tegra3-Arm7-Debug',
      #'Test-Android-Nexus7-Tegra3-Arm7-Release',
      #'Test-Android-NexusS-SGX540-Arm7-Debug',
      #'Test-Android-NexusS-SGX540-Arm7-Release',
      #'Test-Android-Xoom-Tegra2-Arm7-Debug',
      #'Test-Android-Xoom-Tegra2-Arm7-Release',
      #'Test-ChromeOS-Alex-GMA3150-x86-Debug',
      #'Test-ChromeOS-Alex-GMA3150-x86-Release',
      #'Test-ChromeOS-Daisy-MaliT604-Arm7-Debug',
      #'Test-ChromeOS-Daisy-MaliT604-Arm7-Release',
      #'Test-ChromeOS-Link-HD4000-x86_64-Debug',
      #'Test-ChromeOS-Link-HD4000-x86_64-Release',
      #'Test-Mac10.6-MacMini4.1-GeForce320M-x86-Debug',
      #'Test-Mac10.6-MacMini4.1-GeForce320M-x86-Release',
      #'Test-Mac10.6-MacMini4.1-GeForce320M-x86_64-Debug',
      #'Test-Mac10.6-MacMini4.1-GeForce320M-x86_64-Release',
      #'Test-Mac10.7-MacMini4.1-GeForce320M-x86-Debug',
      #'Test-Mac10.7-MacMini4.1-GeForce320M-x86-Release',
      #'Test-Mac10.7-MacMini4.1-GeForce320M-x86_64-Debug',
      #'Test-Mac10.7-MacMini4.1-GeForce320M-x86_64-Release',
      #'Test-Mac10.8-MacMini4.1-GeForce320M-x86-Debug',
      #'Test-Mac10.8-MacMini4.1-GeForce320M-x86-Release',
      #'Test-Mac10.8-MacMini4.1-GeForce320M-x86_64-Debug',
      #'Test-Mac10.8-MacMini4.1-GeForce320M-x86_64-Release',
      #'Test-Ubuntu12-ShuttleA-ATI5770-x86-Debug',
      #'Test-Ubuntu12-ShuttleA-ATI5770-x86-Release',
      #'Test-Ubuntu12-ShuttleA-ATI5770-x86_64-Debug',
      #'Test-Ubuntu12-ShuttleA-ATI5770-x86_64-Release',
      #'Test-Ubuntu12-ShuttleA-HD2000-x86_64-Release-Valgrind',
      'Test-Ubuntu12-ShuttleA-NoGPU-x86_64-Debug',
      #'Test-Ubuntu13-ShuttleA-HD2000-x86_64-Debug-ASAN',
      #'Test-Win7-ShuttleA-HD2000-x86-Debug',
      #'Test-Win7-ShuttleA-HD2000-x86-Debug-ANGLE',
      #'Test-Win7-ShuttleA-HD2000-x86-Debug-DirectWrite',
      #'Test-Win7-ShuttleA-HD2000-x86-Release',
      #'Test-Win7-ShuttleA-HD2000-x86-Release-ANGLE',
      #'Test-Win7-ShuttleA-HD2000-x86-Release-DirectWrite',
      #'Test-Win7-ShuttleA-HD2000-x86_64-Debug',
      #'Test-Win7-ShuttleA-HD2000-x86_64-Release',
    ]
    skia_git_solution = '{ "name": "skia", "url": "https://skia.googlesource.com/skia.git", "custom_deps": {},"custom_vars": {},},'
    if self._builder_name in presync_bots:
      gclient_spec += skia_git_solution
    ############################################################################

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
          manually_grab_svn_rev=True,
          force=True,
          delete_unversioned_trees=True)
      got_revision = gclient_utils.GetCheckedOutRevision()
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
          manually_grab_svn_rev=True,
          force=True,
          delete_unversioned_trees=True,
          jobs=1)
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

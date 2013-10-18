#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia GM executable. """

from build_step import BuildStep
import build_step
import os
import sys


JSON_SUMMARY_FILENAME = 'actual-results.json'
OVERRIDE_IGNORED_TESTS_FILE = 'ignored-tests.txt'


class RunGM(BuildStep):
  def _GetAdditionalTestsToIgnore(self):
    """Parse the OVERRIDE_IGNORED_TESTS_FILE, and return any tests listed there
    as an list.  If the file is empty or nonexistent, return an empty list.

    See https://code.google.com/p/skia/issues/detail?id=1600#c4
    """
    tests = []
    override_ignored_tests_path = os.path.join(
        self._gm_expected_dir, os.pardir, OVERRIDE_IGNORED_TESTS_FILE)
    try:
      with open(override_ignored_tests_path) as f:
        for line in f.readlines():
          line = line.strip()
          if not line:
            continue
          if line.startswith('#'):
            continue
          tests.append(line)
    except IOError:
      print ('override_ignored_tests_path %s does not exist' %
             override_ignored_tests_path)
      return []
    print ('Found these tests to ignore at override_ignored_tests_path %s: %s' %
           (override_ignored_tests_path, tests))
    return tests

  def _Run(self):
    output_dir = os.path.join(self._device_dirs.GMActualDir(),
                              self._builder_name)
    cmd = ['--verbose',
           '--writeChecksumBasedFilenames',
           # Don't bother writing out image files that match our expectations--
           # we know that previous runs have already uploaded those!
           '--mismatchPath', output_dir,
           '--missingExpectationsPath', output_dir,
           '--writeJsonSummaryPath', os.path.join(output_dir,
                                                  JSON_SUMMARY_FILENAME),
           '--ignoreErrorTypes',
               'IntentionallySkipped', 'MissingExpectations',
               'ExpectationsMismatch',
           '--resourcePath', self._device_dirs.ResourceDir(),
           ] + self._gm_args

    device_gm_expectations_path = self._flavor_utils.DevicePathJoin(
        self._device_dirs.GMExpectedDir(), build_step.GM_EXPECTATIONS_FILENAME)
    if self._flavor_utils.DevicePathExists(device_gm_expectations_path):
      cmd.extend(['--readPath', device_gm_expectations_path])

    additional_tests_to_ignore = self._GetAdditionalTestsToIgnore()
    if additional_tests_to_ignore:
      cmd.extend(['--ignoreTests'] + additional_tests_to_ignore)

    if 'Xoom' in self._builder_name:
      # The Xoom's GPU will crash on some tests if we don't use this flag.
      # http://code.google.com/p/skia/issues/detail?id=1434
      cmd.append('--resetGpuContext')
    if sys.platform == 'darwin':
      # msaa16 is flaky on Macs (driver bug?) so we skip the test for now
      cmd.extend(['--config', 'defaults', '~msaa16'])
    elif ('RazrI' in self._builder_name or
          'Nexus10' in self._builder_name or
          'GalaxyNexus' in self._builder_name or
          'Nexus4' in self._builder_name):
      cmd.extend(['--config', 'defaults', 'msaa4'])
    elif (not 'NoGPU' in self._builder_name and
          not 'ChromeOS' in self._builder_name):
      cmd.extend(['--config', 'defaults', 'msaa16'])
    if 'ZeroGPUCache' in self._builder_name:
      cmd.extend(['--gpuCacheSize', '0', '0', '--config', 'gpu'])
    self._flavor_utils.RunFlavoredCmd('gm', cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunGM))

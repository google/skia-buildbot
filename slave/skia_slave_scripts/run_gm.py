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


class RunGM(BuildStep):
  def _Run(self):
    device_gm_expectations_path = self._flavor_utils.DevicePathJoin(
        self._device_dirs.GMExpectedDir(), build_step.GM_EXPECTATIONS_FILENAME)
    output_dir = os.path.join(self._device_dirs.GMActualDir(),
                              self._gm_image_subdir)
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
           '--readPath', device_gm_expectations_path,
           # TODO(borenet): Re-enable --resourcePath when the test passes.
           #'--resourcePath', self._device_dirs.ResourceDir(),
           ] + self._gm_args
    # msaa16 is flaky on Macs (driver bug?) so we skip the test for now
    if sys.platform == 'darwin':
      cmd.extend(['--config', 'defaults', '~msaa16'])
    elif hasattr(self, '_device') and self._device in ['razr_i', 'nexus_10',
                                                       'galaxy_nexus']:
      cmd.extend(['--config', 'defaults', 'msaa4'])
    elif (not 'NoGPU' in self._builder_name and
          not 'ChromeOS' in self._builder_name):
      cmd.extend(['--config', 'defaults', 'msaa16'])
    self._flavor_utils.RunFlavoredCmd('gm', cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunGM))

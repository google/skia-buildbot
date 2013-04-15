#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia GM executable. """

from build_step import BuildStep
import os
import sys


JSON_SUMMARY_FILENAME = 'actual-results.json'


class RunGM(BuildStep):
  def _Run(self):
    output_dir = os.path.join(self._device_dirs.GMDir(), self._gm_image_subdir)
    cmd = ['--verbose',
           '--writePath', output_dir,
           '--writeJsonSummaryPath', os.path.join(output_dir,
                                                  JSON_SUMMARY_FILENAME),
           '--ignoreErrorTypes',
               'IntentionallySkipped', 'MissingExpectations',
               'ExpectationsMismatch',
           '--readPath', self._device_dirs.GMExpectedDir(),
           '--resourcePath', self._device_dirs.ResourceDir(),
           ] + self._gm_args
    # msaa16 is flaky on Macs (driver bug?) so we skip the test for now
    if sys.platform == 'darwin':
      cmd.extend(['--excludeConfig', 'msaa16'])
    self.RunFlavoredCmd('gm', cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunGM))

#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia skimage executable. """

from build_step import BuildStep, BuildStepFailure, GM_EXPECTATIONS_FILENAME
# builder_name_schema must be imported after build_step so the PYTHONPATH will
# be set properly to import it.
import builder_name_schema
import run_gm
import sys

class RunDecodingTests(BuildStep):
  def _Run(self):
    cmd = ['-r', self._device_dirs.SKImageInDir(), '--noreencode']

    subdir = builder_name_schema.GetWaterfallBot(self._builder_name)

    # Read expectations, which were downloaded/copied to the device.
    expectations_file = self._flavor_utils.DevicePathJoin(
      self._device_dirs.SKImageExpectedDir(), subdir,
      GM_EXPECTATIONS_FILENAME)

    have_expectations = self._flavor_utils.DevicePathExists(expectations_file)
    if have_expectations:
      cmd.extend(['--readExpectationsPath', expectations_file])

    # Write the expectations file, in case any did not match.
    device_subdir = self._flavor_utils.DevicePathJoin(
        self._device_dirs.SKImageOutDir(), subdir)
    self._flavor_utils.CreateCleanDeviceDirectory(device_subdir)
    output_expectations_file = self._flavor_utils.DevicePathJoin(
        device_subdir, run_gm.JSON_SUMMARY_FILENAME)

    cmd.extend(['--createExpectationsPath', output_expectations_file])

    # Draw any mismatches to the same folder as the output json.
    cmd.extend(['--mismatchPath', self._device_dirs.SKImageOutDir()])

    self._flavor_utils.RunFlavoredCmd('skimage', cmd)

    # If there is no expectations file, still run the tests, and then report a
    # failure. Then we'll know to update the expectations with the results of
    # running the tests.
    if not have_expectations:
      raise BuildStepFailure("Missing expectations file " + expectations_file)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDecodingTests))

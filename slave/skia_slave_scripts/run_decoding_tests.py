#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia skimage executable. """

from build_step import BuildStep, BuildStepWarning
# builder_name_schema must be imported after build_step so the PYTHONPATH will
# be set properly to import it.
import builder_name_schema
import sys

class RunDecodingTests(BuildStep):
  def _Run(self):
    cmd = ['-r', self._device_dirs.SKImageInDir(), '--noreencode']

    expectations_name = (builder_name_schema.GetWaterfallBot(self._builder_name)
                         + '.json')

    # Read expectations, which were downloaded/copied to the device.
    expectations_file = self._flavor_utils.DevicePathJoin(
      self._device_dirs.SKImageExpectedDir(),
      expectations_name)

    have_expectations = self._flavor_utils.DevicePathExists(expectations_file)
    if have_expectations:
      cmd.extend(['--readExpectationsPath', expectations_file])

    # Write the expectations file, in case any did not match.
    output_expectations_file = self._flavor_utils.DevicePathJoin(
        self._device_dirs.SKImageOutDir(),
        expectations_name)

    cmd.extend(['--createExpectationsPath', output_expectations_file])

    # Draw any mismatches to the same folder as the output json.
    cmd.extend(['--mismatchPath', self._device_dirs.SKImageOutDir()])

    self._flavor_utils.RunFlavoredCmd('skimage', cmd)

    # If there is no expectations file, still run the tests, and then raise a
    # warning. Then we'll know to update the expectations with the results of
    # running the tests.
    # TODO(scroggo): Make this a failure once all builders have expectations.
    if not have_expectations:
      raise BuildStepWarning("Missing expectations file " + expectations_file)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDecodingTests))

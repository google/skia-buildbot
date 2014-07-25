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
    cmd = ['-r', self._device_dirs.SKImageInDir(), '--noreencode',
           '--writeChecksumBasedFilenames', '--config', '8888']

    waterfall_name = builder_name_schema.GetWaterfallBot(self._builder_name)

    # Read expectations, which were downloaded/copied to the device.
    # If this bot is a trybot, read the expected results of the waterfall bot.
    expectations_file = self._flavor_utils.DevicePathJoin(
      self._device_dirs.SKImageExpectedDir(), waterfall_name,
      GM_EXPECTATIONS_FILENAME)

    have_expectations = self._flavor_utils.DevicePathExists(expectations_file)
    if have_expectations:
      cmd.extend(['--readExpectationsPath', expectations_file])

    # Write the expectations file, in case any did not match.
    device_subdir = self._flavor_utils.DevicePathJoin(
        self._device_dirs.SKImageOutDir(), self._builder_name)
    self._flavor_utils.CreateCleanDeviceDirectory(device_subdir)
    output_expectations_file = self._flavor_utils.DevicePathJoin(
        device_subdir, run_gm.JSON_SUMMARY_FILENAME)

    cmd.extend(['--createExpectationsPath', output_expectations_file])

    # Draw any mismatches to a folder inside SKImageOutDir.
    image_out_dir = self._flavor_utils.DevicePathJoin(
        self._device_dirs.SKImageOutDir(), 'images')
    self._flavor_utils.CreateCleanDeviceDirectory(image_out_dir)
    cmd.extend(['--mismatchPath', image_out_dir])

    self._flavor_utils.RunFlavoredCmd('skimage', cmd)

    # If there is no expectations file, still run the tests, and then report a
    # failure. Then we'll know to update the expectations with the results of
    # running the tests.
    # TODO(scroggo): Skipping the TSAN bot, where we'll never have
    # expectations. A better way might be to have empty expectations. See
    # https://code.google.com/p/skia/issues/detail?id=1711
    if not have_expectations and not 'TSAN' in self._builder_name:
      name = self._builder_name
      msg = (('Missing expectations file %s.\n'
              'In order to blindly use the actual results as the expectations,'
              '\nrun the following commands once UploadSKImageResults '
              'succeeds:\n') % expectations_file)
      cmds = (('$ gsutil cp -R gs://chromium-skia-gm/skimage/actuals/%s '
               'expectations/skimage/%s\n') % (name, waterfall_name))

      cmds += (('$ mv expectations/skimage/%s/actual-results.json '
                'expectations/skimage/%s/%s\n') %
               (waterfall_name, waterfall_name, GM_EXPECTATIONS_FILENAME))
      cmds += '\nThen check in using git.\n'
      raise BuildStepFailure(msg + cmds)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDecodingTests))

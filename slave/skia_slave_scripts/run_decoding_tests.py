#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia skimage executable. """

from build_step import BuildStep
from utils.gs_utils import TIMESTAMP_COMPLETED_FILENAME
import sys

class RunDecodingTests(BuildStep):
  def _Run(self):
    image_dir = self._device_dirs.SKImageInDir()

    # Skip the time stamp file, if present.
    # On desktops, the timestamp file will be present and will cause the
    # test to report a failure, so supply a list of files.
    # On Android, the timestamp is not present, so the list is unnecessary,
    # and is actually too long, so just supply the directory.
    timestamp_full_path = self.DevicePathJoin(image_dir,
                                              TIMESTAMP_COMPLETED_FILENAME)
    if self.DevicePathExists(timestamp_full_path):
      images = [self.DevicePathJoin(image_dir, filename)
                for filename in self.DeviceListDir(image_dir)
                if not filename == TIMESTAMP_COMPLETED_FILENAME]
    else:
      images = [image_dir]

    cmd = ['-r']
    cmd.extend(images)

    if self._gm_image_subdir is not None:
      expectations_name = self._gm_image_subdir + '.json'

      # Read expectations, which were downloaded/copied to the device.
      expectations_file = self.DevicePathJoin(
        self._device_dirs.SKImageExpectedDir(),
        expectations_name)

      if self.DevicePathExists(expectations_file):
        cmd.extend(['--readExpectationsPath', expectations_file])

    # Write the expectations file, in case any did not match.
    output_expectations_file = self.DevicePathJoin(
        self._device_dirs.SKImageOutDir(),
        expectations_name)

    cmd.extend(['--createExpectationsPath', output_expectations_file])

    # Draw any mismatches to the same folder as the output json.
    cmd.extend(['--mismatchPath', self._device_dirs.SKImageOutDir()])

    self.RunFlavoredCmd('skimage', cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDecodingTests))

#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia skimage executable. """

from build_step import BuildStep
from utils.gs_utils import TIMESTAMP_COMPLETED_FILENAME
import os
import sys

class RunDecodingTests(BuildStep):
  def _Run(self):
    image_dir = self._device_dirs.SKImageInDir()

    # Skip the time stamp file.
    images = [os.path.join(image_dir, filename)
              for filename in os.listdir(image_dir)
              if not filename == TIMESTAMP_COMPLETED_FILENAME]
    cmd = ['-r']
    cmd.extend(images)

    expectations_name = self._gm_image_subdir + '.json'

    # Read expectations, which were downloaded/copied to the device.
    expectations_file = os.path.join(self._device_dirs.SKImageExpectedDir(),
                                     expectations_name)

    if os.path.exists(expectations_file):
      cmd.extend(['--readExpectationsPath', expectations_file])

    # Write the expectations file, in case any did not match.
    output_expectations_file = os.path.join(self._device_dirs.SKImageOutDir(),
                                            expectations_name)
    cmd.extend(['--createExpectationsPath', output_expectations_file])

    # Draw any mismatches to the same folder as the output json.
    cmd.extend(['--mismatchPath', self._device_dirs.SKImageOutDir()])

    self.RunFlavoredCmd('skimage', cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDecodingTests))

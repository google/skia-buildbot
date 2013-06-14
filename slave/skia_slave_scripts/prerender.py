#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Test builders but not
    Bench builders. """

from build_step import BuildStep
import os
import shutil
import sys


class PreRender(BuildStep):
  def _Run(self):
    # Push the GM expectations to the device.
    # TODO(borenet) Enable expectations once we're using checksums.  It will
    # take too long to push the expected images, but the checksums will be
    # much faster.
    self.CreateCleanDirectory(self._device_dirs.GMExpectedDir())
    #self.CopyDirectoryContentsToDevice(self._gm_expected_dir,
    #                                   self._device_dirs.GMExpectedDir())

    # Prepare directory to hold GM actuals.
    if os.path.exists(self._gm_actual_dir):
      print 'Removing %s' % self._gm_actual_dir
      shutil.rmtree(self._gm_actual_dir)
    print 'Creating %s' % self._gm_actual_dir
    os.makedirs(self._gm_actual_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PreRender))

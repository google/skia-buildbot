#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Pulls the directory with render results from the Android device. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from postrender import PostRender
from utils import android_utils
import posixpath
import sys


class AndroidPostRender(AndroidBuildStep, PostRender):
  def _Run(self):
    super(AndroidPostRender, self)._Run()

    android_utils.RunADB(self._serial, ['pull', posixpath.join(
                                            self._device_dirs.GMActualDir(),
                                            self._gm_image_subdir),
                                        self._gm_actual_dir])
    android_utils.RunADB(self._serial, ['pull', self._device_dirs.SKPOutDir(),
                                        self._gm_actual_dir])
    android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                        self._device_dirs.GMActualDir()])
    android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                        self._device_dirs.SKPOutDir()])

    # Pull skimage results from device:
    android_utils.RunADB(self._serial, ['pull',
                                        self._device_dirs.SKImageOutDir(),
                                        self._skimage_out_dir])

    # And remove them.
    android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                        self._device_dirs.SKImageOutDir()])

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidPostRender))

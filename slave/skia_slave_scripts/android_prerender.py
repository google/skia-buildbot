#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Test builders but not
    Bench builders. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from prerender import PreRender
from utils import android_utils
import posixpath
import sys


class AndroidPreRender(AndroidBuildStep, PreRender):
  def _Run(self):
    super(AndroidPreRender, self)._Run()

    try:
      android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                          self._device_dirs.GMDir()])
    except Exception:
      pass
    try:
      android_utils.RunADB(self._serial, ['shell', 'rm', '-r',
                                          self._device_dirs.SKPOutDir()])
    except Exception:
      pass
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                        posixpath.join(
                                            self._device_dirs.GMDir(),
                                            self._gm_image_subdir)])
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p',
                                        self._device_dirs.SKPOutDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidPreRender))

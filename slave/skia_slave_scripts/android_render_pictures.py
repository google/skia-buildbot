#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
from utils import android_utils
import glob
import os
import sys


BINARY_NAME = 'render_pictures'


class AndroidRenderPictures(RenderPictures, AndroidBuildStep):
  def _PushSKPSources(self, serial):
    """ Push the skp directory full of .skp's to the Android device.

    serial: string indicating the serial number of the target device.
    """
    try:
      android_utils.RunADB(serial, ['shell', 'rm', '-r', '%s' % \
                                    self._device_dirs.SKPDir()])
    except Exception:
      pass
    try:
      android_utils.RunADB(serial, ['shell', 'rm', '-r', '%s' % \
                                    self._device_dirs.SKPOutDir()])
    except Exception:
      pass
    android_utils.RunADB(serial, ['shell', 'mkdir', '-p','%s' % \
                                  self._device_dirs.SKPDir()])
    android_utils.RunADB(serial, ['shell', 'mkdir', '-p', '%s' % \
                                  self._device_dirs.SKPOutDir()])
    # Push each skp individually, since adb doesn't let us use wildcards
    for skp in glob.glob(os.path.join(self._skp_dir, '*.skp')):
      android_utils.RunADB(serial, ['push', skp, self._device_dirs.SKPDir()])

  def _PullSKPResults(self, serial):
    android_utils.RunADB(serial, ['pull', self._device_dirs.SKPOutDir(),
                                  self._gm_actual_dir])

  def DoRenderPictures(self, verify_args):
    args = self._PictureArgs(self._device_dirs.SKPDir(),
                             self._device_dirs.SKPOutDir(),
                             'bitmap')
    android_utils.RunShell(self._serial, [BINARY_NAME] + args + verify_args)

  def _Run(self):
    # For this step, we assume that we run *after* RunGM and *before*
    # UploadGMResults.  This needs to be the case, because RunGM clears the
    # output directory before it begins, and because we want the results from
    # this step to be uploaded with the GM results.
    self._PushSKPSources(self._serial)
    try:
      super(AndroidRenderPictures, self)._Run()
    finally:
      self._PullSKPResults(self._serial)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRenderPictures))


#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
from utils import misc
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
      misc.RunADB(serial, ['shell', 'rm', '-r', '%s' % self._android_skp_dir])
    except:
      pass
    try:
      misc.RunADB(serial,
                  ['shell', 'rm', '-r', '%s' % self._android_skp_out_dir])
    except:
      pass
    misc.RunADB(serial, ['shell', 'mkdir', '-p', '%s' % self._android_skp_dir])
    misc.RunADB(serial,
                ['shell', 'mkdir', '-p', '%s' % self._android_skp_out_dir])
    misc.RunADB(serial, ['shell', 'chmod', '777',
                         '%s' % self._android_skp_out_dir])
    # Push each skp individually, since adb doesn't let us use wildcards
    for skp in glob.glob(os.path.join(self._skp_dir, '*.skp')):
      misc.RunADB(serial, ['push', skp, self._android_skp_dir])

  def _PullSKPResults(self, serial):
    misc.RunADB(serial, ['pull', self._android_skp_out_dir,
                         self._gm_actual_dir])

  def _Run(self, args):
    # For this step, we assume that we run *after* RunGM and *before*
    # UploadGMResults.  This needs to be the case, because RunGM clears the
    # output directory before it begins, and because we want the results from
    # this step to be uploaded with the GM results.
    self._PushSKPSources(self._serial)
    args = self._PictureArgs(self._android_skp_dir, self._android_skp_out_dir,
                             'bitmap')
    misc.Run(self._serial, BINARY_NAME, args)
    self._PullSKPResults(self._serial)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidRenderPictures))


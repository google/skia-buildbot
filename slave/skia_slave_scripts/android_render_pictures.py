#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
from utils import android_utils
import sys


BINARY_NAME = 'render_pictures'


class AndroidRenderPictures(RenderPictures, AndroidBuildStep):
  def DoRenderPictures(self, verify_args):
    args = self._PictureArgs(self._device_dirs.SKPDir(),
                             self._device_dirs.SKPOutDir(),
                             'bitmap')
    android_utils.RunShell(self._serial, [BINARY_NAME] + args + verify_args)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRenderPictures))


#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
import sys


class AndroidRenderPictures(AndroidBuildStep, RenderPictures):
  pass

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRenderPictures))

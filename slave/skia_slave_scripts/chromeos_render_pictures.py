#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
import sys


class ChromeOSRenderPictures(ChromeOSBuildStep, RenderPictures):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRenderPictures))

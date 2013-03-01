#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
import sys


# pylint: disable=W0231
class ChromeOSRenderPictures(ChromeOSBuildStep, RenderPictures):
  def __init__(self, timeout=6400, **kwargs):
    ChromeOSBuildStep.__init__(self, timeout=timeout, **kwargs)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRenderPictures))

#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pdfs executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from render_pdfs import RenderPdfs
import sys


class AndroidRenderPdfs(AndroidBuildStep, RenderPdfs):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRenderPdfs))

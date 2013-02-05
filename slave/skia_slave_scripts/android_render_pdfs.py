#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pdfs executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from render_pdfs import RenderPdfs
from utils import android_utils
import sys


BINARY_NAME = 'render_pdfs'


class AndroidRenderPdfs(RenderPdfs, AndroidBuildStep):
  def DoRenderPdfs(self):
    args = self._PdfArgs(self._device_dirs.SKPDir())
    android_utils.RunShell(self._serial, [BINARY_NAME] + args)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRenderPdfs))


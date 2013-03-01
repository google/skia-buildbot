#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pdfs executable. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from render_pdfs import RenderPdfs
import sys


class ChromeOSRenderPdfs(ChromeOSBuildStep, RenderPdfs):
  def __init__(self, args, attempts=1, timeout=4800, **kwargs):
    super(ChromeOSRenderPdfs, self).__init__(args, attempts=attempts,
                                             timeout=timeout, **kwargs)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRenderPdfs))


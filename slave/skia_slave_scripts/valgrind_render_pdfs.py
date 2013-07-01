#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pdfs executable. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from render_pdfs import RenderPdfs
import sys


class ValgrindRenderPdfs(ValgrindBuildStep, RenderPdfs):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindRenderPdfs))

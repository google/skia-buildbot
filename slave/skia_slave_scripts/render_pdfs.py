#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pdfs executable. """

from utils import shell_utils
from build_step import BuildStep
import os
import sys

class RenderPdfs(BuildStep):

  def _PdfArgs(self, skp_dir):
    return [skp_dir]

  def DoRenderPdfs(self):
    # Render the pdfs in memory.
    skp_dir = os.path.join(os.pardir, 'skp')
    args = self._PdfArgs(skp_dir)
    cmd = [self._PathToBinary('render_pdfs')] + args
    shell_utils.Bash(cmd)

  def _Run(self):
    self.DoRenderPdfs()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderPdfs))

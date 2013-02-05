#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pdfs executable. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from render_pdfs import RenderPdfs
from utils import ssh_utils
import sys


BINARY_NAME = 'skia_render_pdfs'


class ChromeOSRenderPdfs(RenderPdfs, ChromeOSBuildStep):
  def __init__(self, args, attempts=1, timeout=4800, **kwargs):
    super(ChromeOSRenderPdfs, self).__init__(args, attempts=attempts,
                                                 timeout=timeout, **kwargs)

  def DoRenderPdfs(self):
    args = self._PdfArgs(self._device_dirs.SKPDir())
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     [BINARY_NAME] + args)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRenderPdfs))


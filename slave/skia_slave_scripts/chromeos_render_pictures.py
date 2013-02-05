#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
from utils import ssh_utils
import sys


BINARY_NAME = 'skia_render_pictures'


class ChromeOSRenderPictures(RenderPictures, ChromeOSBuildStep):
  def __init__(self, timeout=4800, **kwargs):
    super(ChromeOSRenderPictures, self).__init__(timeout=timeout, **kwargs)

  def DoRenderPictures(self, verify_args):
    args = self._PictureArgs(self._device_dirs.SKPDir(),
                             self._device_dirs.SKPOutDir(), 'bitmap')
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     [BINARY_NAME] + args + verify_args)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRenderPictures))


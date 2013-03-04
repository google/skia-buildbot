#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from build_step import BuildStep
import os
import sys


DEFAULT_TILE_X = 256
DEFAULT_TILE_Y = 256


class RenderPictures(BuildStep):
  def DoRenderPictures(self, args, device='bitmap', write_images=True):
    cmd = [self._device_dirs.SKPDir(), '--device', device,
           '--mode', 'tile', str(DEFAULT_TILE_X), str(DEFAULT_TILE_Y)]
    cmd.extend(args)
    if not hasattr(self, '_device') and not os.name == 'nt' and \
        not hasattr(self, '_ssh_host'):
      # For now, skip --validate and writing images on Android and ChromeOS,
      # since some of our pictures are too big to fit in memory, and the images
      # take too long to transfer.
      # Also skip --validate on Windows, where it is currently failing.
      cmd.append('--validate')
      if write_images:
        cmd.extend(['-w', self._device_dirs.SKPOutDir()])
    self.RunFlavoredCmd('render_pictures', cmd)

  def _Run(self):
    self.DoRenderPictures([])
    self.DoRenderPictures(['--bbh', 'grid', str(DEFAULT_TILE_X),
                           str(DEFAULT_TILE_X), '--clone', '1'])
    self.DoRenderPictures(['--bbh', 'rtree', '--clone', '2'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderPictures))

#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from utils import shell_utils
from build_step import BuildStep
import os
import shutil
import sys
import tempfile


class RenderPictures(BuildStep):
  def _PictureArgs(self, skp_dir, out_dir, device):
    args = [skp_dir, '--device', device,
            '--mode', 'tile', str(self.TILE_X), str(self.TILE_Y)]
    if not hasattr(self, 'device'):
      # For now, only run --validate when not on Android, since some of our
      # pictures are too big to fit in memory.
      args.append('--validate')
    return args

  def DoRenderPictures(self, verify_args):
    # Render the pictures into a temporary directory.
    temp_dir = tempfile.mkdtemp()
    skp_dir = os.path.join(os.pardir, 'skp')
    args = self._PictureArgs(skp_dir, temp_dir, 'bitmap')
    cmd = [self._PathToBinary('render_pictures')] + args + verify_args
    shell_utils.Bash(cmd)
    # Copy the files into gm_actual_dir, prepending 'skp_' to the filename
    filepaths = os.listdir(temp_dir)
    for filepath in filepaths:
      if not os.path.isdir(filepath):
        out_file = os.path.join(self._gm_actual_dir, filepath)
        shutil.copyfile(os.path.join(temp_dir, filepath), out_file)
    shutil.rmtree(temp_dir)

  def _Run(self):
    self.DoRenderPictures([])
    self.DoRenderPictures(['--clone', '1'])
    self.DoRenderPictures(['--clone', '2'])
    self.DoRenderPictures(['--bbh', 'grid', '256', '256'])
    self.DoRenderPictures(['--bbh', 'rtree'])

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderPictures))

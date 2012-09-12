#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import misc
from build_step import BuildStep
import os
import shutil
import sys
import tempfile

class RenderPictures(BuildStep):
  def _Run(self, args):
    # Render the pictures into a temporary directory.
    temp_dir = tempfile.mkdtemp()
    skp_dir = os.path.join(os.pardir, 'skp')
    cmd = [self._PathToBinary('render_pictures'), skp_dir, temp_dir]
    misc.Bash(cmd)
    # Copy the files into gm_actual_dir, prepending 'skp_' to the filename
    filepaths = os.listdir(temp_dir)
    for filepath in filepaths:
      if not os.path.isdir(filepath):
        out_file = os.path.join(self._gm_actual_dir, filepath)
        shutil.copyfile(os.path.join(temp_dir, filepath), out_file)
    shutil.rmtree(temp_dir)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(RenderPictures))


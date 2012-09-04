#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import misc
from build_step import BuildStep
import os
import sys

class BenchPictures(BuildStep):
  def _Run(self, args):
    cmd = [self._PathToBinary('bench_pictures'),
           os.path.join(os.pardir, 'skp')]
    misc.Bash(cmd)
    # TODO(borenet): Grab the output and prepare it for GenerateBenchGraphs

if '__main__' == __name__:
  sys.exit(BuildStep.Run(BenchPictures))


#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compare the generated GM images to the baselines """

from utils import misc
from build_step import BuildStep
import sys

class CompareGMs(BuildStep):
  def _Run(self, args):
    cmd = [self._PathToBinary('skdiff'),
           '--listfilenames',
           '--nodiffs',
           '--nomatch', 'README',
           '--failonresult', 'DifferentPixels',
           '--failonresult', 'DifferentSizes',
           '--failonresult', 'DifferentOther',
           '--failonresult', 'Unknown',
           self._gm_expected_dir,
           self._gm_actual_dir,
           ]
    return misc.Bash(cmd)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(CompareGMs))
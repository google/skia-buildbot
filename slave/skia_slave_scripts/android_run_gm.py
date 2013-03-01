#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run GM on an Android device. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from run_gm import RunGM
import sys


class AndroidRunGM(AndroidBuildStep, RunGM):
  def _Run(self):
    self._gm_args.append('--nopdf')
    RunGM._Run(self)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRunGM))

#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia GM executable. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from run_gm import RunGM
import sys


class ChromeOSRunGM(ChromeOSBuildStep, RunGM):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRunGM))

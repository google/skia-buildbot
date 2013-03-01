#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from chromeos_build_step import ChromeOSBuildStep
from bench_pictures import BenchPictures
from build_step import BuildStep
import sys


class ChromeOSBenchPictures(ChromeOSBuildStep, BenchPictures):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSBenchPictures))

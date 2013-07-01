#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from bench_pictures import BenchPictures
import sys


class ValgrindBenchPictures(ValgrindBuildStep, BenchPictures):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindBenchPictures))

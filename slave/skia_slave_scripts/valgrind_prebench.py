#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Bench builders but not
    Test builders. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from prebench import PreBench
import sys


class ValgrindPreBench(ValgrindBuildStep, PreBench):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindPreBench))

#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia skimage executable. """

from build_step import BuildStep
from valgrind_build_step import ValgrindBuildStep
from run_decoding_tests import RunDecodingTests
import sys


class ValgrindRunDecodingTests(ValgrindBuildStep, RunDecodingTests):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindRunDecodingTests))

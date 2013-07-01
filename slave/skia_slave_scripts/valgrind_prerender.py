#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Test builders but not
    Bench builders. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from prerender import PreRender
import sys


class ValgrindPreRender(ValgrindBuildStep, PreRender):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindPreRender))

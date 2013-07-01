#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Pulls the directory with render results from the Valgrind device. """

from valgrind_build_step import ValgrindBuildStep
from build_step import BuildStep
from postrender import PostRender
import sys


class ValgrindPostRender(ValgrindBuildStep, PostRender):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ValgrindPostRender))

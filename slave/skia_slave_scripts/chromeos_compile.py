#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from compile import Compile
import sys


class ChromeOSCompile(Compile, ChromeOSBuildStep):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSCompile))

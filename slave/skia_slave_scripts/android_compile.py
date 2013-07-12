#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step for Android """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from compile import Compile
import sys


class AndroidCompile(Compile, AndroidBuildStep):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidCompile))

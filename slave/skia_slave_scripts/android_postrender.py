#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Pulls the directory with render results from the Android device. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from postrender import PostRender
import sys


class AndroidPostRender(AndroidBuildStep, PostRender):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidPostRender))

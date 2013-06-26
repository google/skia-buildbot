#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run DecodingTests on an Android device. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from run_decoding_tests import RunDecodingTests
import sys


class AndroidRunDecodingTests(AndroidBuildStep, RunDecodingTests):
  pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidRunDecodingTests))

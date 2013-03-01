#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from run_tests import RunTests
import sys
import threading


class AndroidRunTests(AndroidBuildStep, RunTests):
  pass


if '__main__' == __name__:
  exitcode = BuildStep.RunBuildStep(AndroidRunTests)
  print 'AndroidRunTests finished with code %d' % exitcode
  print 'Threads still running:\n%s' % threading.enumerate()
  sys.exit(exitcode)

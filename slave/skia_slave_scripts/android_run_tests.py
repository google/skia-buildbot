#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from utils import android_utils
import sys
import threading


class AndroidRunTests(AndroidBuildStep):
  def _Run(self):
    android_utils.RunSkia(self._serial, ['tests'], stop_shell=self._has_root,
                          use_intent=(not self._has_root))
    print 'AndroidRunTests process done...'
    print 'Threads still running:\n%s' % threading.enumerate()


if '__main__' == __name__:
  exitcode = BuildStep.RunBuildStep(AndroidRunTests)
  print 'AndroidRunTests finished with code %d' % exitcode
  print 'Threads still running:\n%s' % threading.enumerate()
  sys.exit(exitcode)

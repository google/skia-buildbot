#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from utils import misc
import sys

class AndroidRunTests(AndroidBuildStep):
  def _Run(self, args):
    serial = misc.GetSerial(self._device)
    misc.Run(serial, 'tests')

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidRunTests))
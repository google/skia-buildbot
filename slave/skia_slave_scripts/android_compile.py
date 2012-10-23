#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step for Android """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from utils import misc
import os
import shlex
import sys

ENV_VAR = 'ANDROID_SDK_ROOT'
ANDROID_SDK_ROOT = '/home/chrome-bot/android-sdk-linux'

class AndroidCompile(AndroidBuildStep):
  def _Run(self):
    if not ENV_VAR in os.environ.keys():
      os.environ[ENV_VAR] = ANDROID_SDK_ROOT
    cmd = [os.path.join(os.pardir, 'android', 'bin', 'android_make'),
           self._args['target'],
           '-d', self._device,
           'BUILDTYPE=%s' % self._configuration,
           ]
    cmd += self._make_flags
    misc.Bash(cmd)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AndroidCompile))

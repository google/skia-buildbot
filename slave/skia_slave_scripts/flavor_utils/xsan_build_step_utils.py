#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for ASAN,TSAN,etc. build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from utils import shell_utils

import os

LLVM_PATH = '/home/chrome-bot/llvm-3.4/Release+Asserts/bin/'

class XsanBuildStepUtils(DefaultBuildStepUtils):
  def Compile(self, target):
    # Run the xsan_build script.
    os.environ['PATH'] = LLVM_PATH + ':' + os.environ['PATH']
    shell_utils.run(['which', 'clang'])
    shell_utils.run(['clang', '--version'])
    os.environ['GYP_DEFINES'] = self._step.args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    cmd = [
        os.path.join('tools', 'xsan_build'),
        self._step.args['sanitizer'],
        target,
        'BUILDTYPE=%s' % self._step.configuration,
    ]

    cmd.extend(self._step.default_make_flags)
    cmd.extend(self._step.make_flags)
    shell_utils.run(cmd)

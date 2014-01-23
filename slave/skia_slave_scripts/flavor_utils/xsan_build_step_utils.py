#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for ASAN,TSAN,etc. build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from utils import shell_utils

import os


class XsanBuildStepUtils(DefaultBuildStepUtils):
  def RunFlavoredCmd(self, app, args):
    # Turn on Leak Sanitizer if we're running Address Sanitizer.
    if self._step.args['sanitizer'] == 'address':
      os.environ['ASAN_OPTIONS'] = 'detect_leaks=1'
      os.environ['LSAN_OPTIONS'] = ('suppressions=' +
                                    os.path.join('tools', 'lsan.supp'))
    DefaultBuildStepUtils.RunFlavoredCmd(self, app, args)

  def Compile(self, target):
    # Run the xsan_build script.
    shell_utils.Bash(['which', 'clang'])
    shell_utils.Bash(['clang', '--version'])
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
    shell_utils.Bash(cmd)

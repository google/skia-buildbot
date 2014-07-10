#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for ASAN,TSAN,etc. build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from py.utils import shell_utils

import os

class XsanBuildStepUtils(DefaultBuildStepUtils):
  def Compile(self, target):
    # Run the xsan_build script.
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

  def RunFlavoredCmd(self, app, args):
    # New versions of ASAN run LSAN by default.  We're not yet clean for that.
    os.environ['ASAN_OPTIONS'] = 'detect_leaks=0'
    # Point TSAN at our suppressions file.
    os.environ['TSAN_OPTIONS'] = 'suppressions=tools/tsan.supp'
    return shell_utils.run([self._PathToBinary(app)] + args)

#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step """

from utils import shell_utils
from build_step import BuildStep
import os
import sys


class Compile(BuildStep):
  def _Run(self):
    if 'VS2012' in self._builder_name:
      os.environ['GYP_MSVS_VERSION'] = '2012'
    os.environ['GYP_DEFINES'] = self._args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    make_cmd = 'make'
    if os.name == 'nt':
      make_cmd = 'make.bat'
    cmd = [make_cmd,
           self._args['target'],
           'BUILDTYPE=%s' % self._configuration,
           ]
    cmd.extend(self._default_make_flags)
    cmd.extend(self._make_flags)
    shell_utils.Bash(cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Compile))

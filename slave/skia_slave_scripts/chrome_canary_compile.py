#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step """

from utils import shell_utils
from build_step import BuildStep
import os
import sys


class ChromeCanaryCompile(BuildStep):
  def __init__(self, timeout=4800, **kwargs):
    super(ChromeCanaryCompile, self).__init__(timeout=timeout, **kwargs)

  def _Run(self):
    if 'VS2012' in self._builder_name:
      os.environ['GYP_MSVS_VERSION'] = '2012'
    os.environ['GYP_DEFINES'] = self._args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    make_cmd = 'ninja'
    cmd = [make_cmd,
           '-C', os.path.join('out', self._configuration),
           self._args['target'],
           ]
    cmd.extend(self._make_flags)
    shell_utils.Bash(cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeCanaryCompile))

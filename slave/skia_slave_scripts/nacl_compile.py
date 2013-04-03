#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step for NaCl build. """

from utils import shell_utils
from build_step import BuildStep
import os
import sys


ENV_VAR = 'NACL_SDK_ROOT'


class NaClCompile(BuildStep):
  def _Run(self):
    os.environ[ENV_VAR] = self._args['nacl_sdk_root']
    make_cmd = os.path.join(os.pardir, 'nacl', 'nacl_make')
    cmd = [make_cmd,
           self._args['target'],
           'BUILDTYPE=%s' % self._configuration,
           ]
    cmd += self._make_flags
    cmd.extend(self._default_make_flags)
    shell_utils.Bash(cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(NaClCompile))

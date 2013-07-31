#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step for NaCl build. """

from default_build_step_utils import DefaultBuildStepUtils
from utils import shell_utils

import os


ENV_VAR = 'NACL_SDK_ROOT'


class NaclBuildStepUtils(DefaultBuildStepUtils):
  def Compile(self, target):
    os.environ[ENV_VAR] = self._step.args['nacl_sdk_root']
    cmd = [os.path.join('platform_tools', 'nacl', 'nacl_make'),
           target, 'BUILDTYPE=%s' % self._step.configuration,
           ]
    cmd.extend(self._step.default_make_flags)
    if os.name != 'nt':
      try:
        ccache = shell_utils.Bash(['which', 'ccache'], echo=False)
        if ccache:
          cmd.append('--use-ccache')
      except Exception:
        pass
    cmd.extend(self._step.make_flags)
    shell_utils.Bash(cmd)

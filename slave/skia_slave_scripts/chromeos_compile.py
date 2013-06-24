#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step """

from utils import shell_utils
from build_step import BuildStep
from slave import slave_utils
import os
import sys


class Compile(BuildStep):
  def _Run(self):
    # Add gsutil to PATH
    gsutil = slave_utils.GSUtilSetup()
    os.environ['PATH'] += os.pathsep + os.path.dirname(gsutil)

    # Run the chromeos_make script.
    make_cmd = os.path.join('platform_tools', 'chromeos', 'bin',
                            'chromeos_make')
    cmd = [make_cmd,
           '-d', self._args['board'],
           self._args['target'],
           'BUILDTYPE=%s' % self._configuration,
           ]

    cmd.extend(self._default_make_flags)
    cmd.extend(self._make_flags)
    shell_utils.Bash(cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Compile))

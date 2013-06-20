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

    # Override the default boto file with one which works with ChromeOS utils.
    cros_boto_file = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                                  os.pardir, os.pardir, 'site_config',
                                  '.boto_cros')

    # Run the chromeos_make script.
    make_cmd = os.path.join('platform_tools', 'chromeos', 'bin',
                            'chromeos_make')
    cmd = [make_cmd,
           '-d', self._args['board'],
           self._args['target'],
           '--cros-boto-file', cros_boto_file,
           'BUILDTYPE=%s' % self._configuration,
           ]
    cmd.extend(self._default_make_flags)
    cmd.extend(self._make_flags)
    shell_utils.Bash(cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(Compile))

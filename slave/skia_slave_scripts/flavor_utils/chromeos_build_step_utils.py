# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for ChromeOS build steps. """

from flavor_utils.ssh_build_step_utils import SshBuildStepUtils
from slave import slave_utils
from py.utils import shell_utils
import os


class ChromeosBuildStepUtils(SshBuildStepUtils):
  def __init__(self, build_step_instance):
    SshBuildStepUtils.__init__(self, build_step_instance)
    self._remote_dir = '/usr/local/skiabot'
    systemtype = 'chromeos-' + self._step.args['board']
    self._build_dir = os.path.join('out', 'config', systemtype)

  def Compile(self, target):
    """ Compile the Skia executables. """
    # Add gsutil to PATH
    gsutil = slave_utils.GSUtilSetup()
    os.environ['PATH'] += os.pathsep + os.path.dirname(gsutil)

    # Run the chromeos_make script.
    make_cmd = os.path.join('platform_tools', 'chromeos', 'bin',
                            'chromeos_make')
    cmd = [make_cmd,
           '-d', self._step.args['board'],
           target,
           'BUILDTYPE=%s' % self._step.configuration,
           ]

    cmd.extend(self._step.default_make_flags)
    cmd.extend(self._step.make_flags)
    shell_utils.run(cmd)

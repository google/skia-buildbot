# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for Valgrind build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from py.utils import shell_utils

import os


class ValgrindBuildStepUtils(DefaultBuildStepUtils):
  def __init__(self, build_step_instance):
    DefaultBuildStepUtils.__init__(self, build_step_instance)
    self._suppressions_file = os.path.join('tools', 'valgrind.supp')

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    cmd = ['valgrind', '--gen-suppressions=all', '--leak-check=no',
           '--track-origins=yes', '--error-exitcode=1']
    if self._suppressions_file:
      cmd.append('--suppressions=%s' % self._suppressions_file)

    cmd.append(self._PathToBinary(app))
    cmd.extend(args)
    return shell_utils.run(cmd)

  def Compile(self, target):
    os.environ['GYP_DEFINES'] = self._step.args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    make_cmd = 'make'
    if os.name == 'nt':
      make_cmd = 'make.bat'
    cmd = [make_cmd,
           target,
           'BUILDTYPE=%s' % self._step.configuration,
           ]
    cmd.extend(self._step.default_make_flags)
    cmd.extend(self._step.make_flags)
    shell_utils.run(cmd)

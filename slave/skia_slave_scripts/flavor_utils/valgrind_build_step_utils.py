# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Valgrind build steps. """

from default_build_step_utils import DefaultBuildStepUtils
from utils import shell_utils

import os


class ValgrindBuildStepUtils(DefaultBuildStepUtils):
  def __init__(self, build_step_instance):
    DefaultBuildStepUtils.__init__(self, build_step_instance)
    classname = self._step.__class__.__name__
    if classname == 'ValgrindRunTests':
      self._suppressions_file = os.path.join('tests', 'valgrind.supp')
    elif classname == 'ValgrindRunGM':
      self._suppressions_file = os.path.join('gm', 'valgrind.supp')
    else:
      self._suppressions_file = None

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    cmd = ['valgrind', '--gen-suppressions=all', '--leak-check=full',
           '--track-origins=yes', '--error-exitcode=1']
    if self._suppressions_file:
      cmd.append('--suppressions=%s' % self._suppressions_file)
    cmd.append(self._PathToBinary(app))
    cmd.extend(args)
    return shell_utils.Bash(cmd)

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
    shell_utils.Bash(cmd)

# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Valgrind build steps. """

from build_step import BuildStep, BuildStepUtils
from utils import shell_utils
import os


class ValgrindBuildStepUtils(BuildStepUtils):
  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    cmd = ['valgrind', '--gen-suppressions=all', '--leak-check=full',
           '--track-origins=yes', '--error-exitcode=1']
    if self._step.suppressions_file:
      cmd.append('--suppressions=%s' % self._step.suppressions_file)
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


class ValgrindBuildStep(BuildStep):
  def __init__(self, suppressions_file=None, **kwargs):
    self._suppressions_file = suppressions_file
    super(ValgrindBuildStep, self).__init__(timeout=12000,
                                            no_output_timeout=9600,**kwargs)
    self._flavor_utils = ValgrindBuildStepUtils(self)

  @property
  def suppressions_file(self):
    return self._suppressions_file

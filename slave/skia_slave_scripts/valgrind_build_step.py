# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Valgrind build steps. """

from build_step import BuildStep
from utils import shell_utils


class ValgrindBuildStep(BuildStep):
  def __init__(self, suppressions_file=None, **kwargs):
    self._suppressions_file = suppressions_file
    super(ValgrindBuildStep, self).__init__(timeout=12000,
                                            no_output_timeout=9600,**kwargs)

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    cmd = ['valgrind', '--gen-suppressions=all', '--leak-check=full',
           '--track-origins=yes', '--error-exitcode=1']
    if self._suppressions_file:
      cmd.append('--suppressions=%s' % self._suppressions_file)
    cmd.append(self._PathToBinary(app))
    cmd.extend(args)
    return shell_utils.Bash(cmd)

#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia buildbot scripts. """

from utils import shell_utils
from build_step import BuildStep
from config_private import SKIA_SVN_BASEURL
import os
import sys


BUILD_DIR_DEPTH = 5


class UpdateScripts(BuildStep):
  def _Run(self):
    buildbot_dir = os.path.join(*[os.pardir for _i in range(BUILD_DIR_DEPTH)])
    os.chdir(buildbot_dir)
    if os.name == 'nt':
      gclient = 'gclient.bat'
      svn = 'svn.bat'
    else:
      gclient = 'gclient'
      svn = 'svn'

    # Sometimes the build slaves "forget" the svn server. To prevent this from
    # occurring, use "svn ls" with --trust-server-cert.
    shell_utils.Bash([svn, 'ls', SKIA_SVN_BASEURL, '--non-interactive',
                      '--trust-server-cert'])
    shell_utils.Bash([gclient, 'sync'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateScripts))

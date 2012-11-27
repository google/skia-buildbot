#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia buildbot scripts. """

from utils import shell_utils
from build_step import BuildStep
import config
import os
import sys


BUILD_DIR_DEPTH = 5


class UpdateScripts(BuildStep):
  def _Run(self):
    buildbot_dir = os.path.join(*[os.pardir for i in range(BUILD_DIR_DEPTH)])
    os.chdir(buildbot_dir)
    if os.name == 'nt':
      gclient = 'gclient.bat'
    else:
      gclient = 'gclient'

    try:
      output = shell_utils.Bash([gclient, 'sync'])
    except Exception:
      if 'Server certificate verification failed' in output:
        # Sometimes the build slaves "forget" the svn server. If the sync step
        # failed for this reason, use "svn ls" to refresh the slave's memory.
        shell_utils.Bash(['svn', 'ls', config.Master.skia_url,
                          '--non-interactive', '--trust-server-cert'])
        shell_utils.Bash([gclient, 'sync'])
      else:
        raise


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateScripts))
#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia buildbot scripts. """

from utils import gclient_utils
from build_step import BuildStep
import os
import sys


# Path to the buildbot slave checkout on this machine.  This variable must be
# defined before the build step is run, since __file__ is a relative path and
# will not be valid after changing directories in BuildStep.__init__().
BUILDBOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                            os.pardir, os.pardir))


class UpdateScripts(BuildStep):
  def __init__(self, attempts=5, **kwargs):
    super(UpdateScripts, self).__init__(attempts=attempts, **kwargs)

  def _Run(self):
    print 'chdir to %s' % BUILDBOT_DIR
    os.chdir(BUILDBOT_DIR)

    gclient_utils.Sync()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateScripts))

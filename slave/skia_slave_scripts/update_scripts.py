#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia buildbot scripts. """

from utils import gclient_utils
from utils import shell_utils
from build_step import BuildStep, BuildStepWarning
import os
import shlex
import sys


# Path to the buildbot slave checkout on this machine.  This variable must be
# defined before the build step is run, since __file__ is a relative path and
# will not be valid after changing directories in BuildStep.__init__().
BUILDBOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                            os.pardir, os.pardir, os.pardir))

BUILDBOT_GIT_URL = 'https://skia.googlesource.com/buildbot.git'


class UpdateScripts(BuildStep):
  def __init__(self, attempts=5, **kwargs):
    super(UpdateScripts, self).__init__(attempts=attempts, **kwargs)

  def _Run(self):
    print 'chdir to %s' % BUILDBOT_DIR
    os.chdir(BUILDBOT_DIR)

    # Be sure that we sync to the most recent commit.
    buildbot_revision = None
    warn_on_exit = False
    try:
      output = shell_utils.run([gclient_utils.GIT, 'ls-remote',
                                BUILDBOT_GIT_URL, '--verify',
                                'refs/heads/master'])
      if output:
        buildbot_revision = shlex.split(output)[0]
    except shell_utils.CommandFailedException:
      pass
    if not buildbot_revision:
      buildbot_revision = 'origin/master'
      warn_on_exit = True

    gclient_utils.Sync(revisions=[('buildbot', buildbot_revision)],
                       verbose=True, force=True)
    print 'Skiabot scripts updated to %s' % gclient_utils.GetCheckedOutHash()

    if warn_on_exit:
      raise BuildStepWarning('Could not determine buildbot revision. Attempted '
                             'to sync to origin/master.')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateScripts))

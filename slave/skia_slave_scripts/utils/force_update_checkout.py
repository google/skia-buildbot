#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Forcibly update the local checkout."""



import os
import shlex
import sys

BUILDBOT_PATH = os.path.realpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)), os.pardir, os.pardir, os.pardir)
)

sys.path.append(os.path.join(BUILDBOT_PATH, 'common'))
sys.path.append(os.path.join(BUILDBOT_PATH, 'site_config'))
sys.path.append(os.path.join(BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'scripts'))

import gclient_utils
from py.utils import git_utils
from py.utils import misc
from py.utils import shell_utils
import skia_vars


BUILDBOT_GIT_URL = skia_vars.GetGlobalVariable('buildbot_git_url')
GOT_REVISION_PATTERN = 'Skiabot scripts updated to %s'


def force_update():
  with misc.ChDir(os.path.join(misc.BUILDBOT_PATH, os.pardir)):
    # Run "gclient" before doing anything else to ensure that we get the
    # necessary stuff installed.
    gclient_utils.GClient()

    # Be sure that we sync to the most recent commit.
    buildbot_revision = None
    try:
      output = git_utils.GetRemoteMasterHash(BUILDBOT_GIT_URL)
      if output:
        buildbot_revision = shlex.split(output)[0]
    except shell_utils.CommandFailedException:
      pass
    if not buildbot_revision:
      buildbot_revision = 'origin/master'

    gclient_utils.Sync(revisions=[('buildbot', buildbot_revision)],
                       verbose=True, force=True)
    got_revision = gclient_utils.GetCheckedOutHash()
    print GOT_REVISION_PATTERN % got_revision

    return gclient_utils.GetCheckedOutHash()


if __name__ == '__main__':
  force_update()

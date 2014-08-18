# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Create a CL to update the SKP version."""


import os
import sys

from build_step import BuildStep
from config_private import SKIA_GIT_URL
from py.utils.git_utils import GIT
from py.utils import git_utils
from py.utils import misc
from py.utils import shell_utils


CHROMIUM_SKIA = 'https://chromium.googlesource.com/skia.git'
COMMIT_MSG = '''Update SKP version to %s

Automatic commit by the RecreateSKPs bot.

TBR=
'''
PATH_TO_SKIA = os.path.join('third_party', 'skia')
SKIA_COMMITTER_EMAIL = 'borenet@google.com'
SKIA_COMMITTER_NAME = 'Eric Boren'

class UpdateSkpVersion(BuildStep):
  def __init__(self, timeout=28800, **kwargs):
    super(UpdateSkpVersion, self).__init__(timeout=timeout, **kwargs)

  def _Run(self):
    with misc.ChDir(PATH_TO_SKIA):
      shell_utils.run([GIT, 'config', '--local', 'user.name',
                       SKIA_COMMITTER_NAME])
      shell_utils.run([GIT, 'config', '--local', 'user.email',
                       SKIA_COMMITTER_EMAIL])
      if CHROMIUM_SKIA in shell_utils.run([GIT, 'remote', '-v']):
        shell_utils.run([GIT, 'remote', 'set-url', 'origin', SKIA_GIT_URL,
                         CHROMIUM_SKIA])

      version_file = 'SKP_VERSION'
      skp_version = self._args.get('skp_version')
      with git_utils.GitBranch(branch_name='update_skp_version',
                               commit_msg=COMMIT_MSG % skp_version,
                               commit_queue=not self._is_try):

        # First, upload a version of the CL with just the SKP version changed.
        with open(version_file, 'w') as f:
          f.write(skp_version)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateSkpVersion))

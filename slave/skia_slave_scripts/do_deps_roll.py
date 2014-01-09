#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the DEPS roll automation script."""


import os
import sys

from build_step import BuildStep
from utils import shell_utils


GIT = 'git.bat' if os.name == 'nt' else 'git'


class DEPSRoll(BuildStep):
  """BuildStep which creates and uploads a DEPS roll CL and sets off trybots."""

  def _Run(self):
    roll_deps_script = os.path.join('third_party', 'skia', 'tools',
                                    'roll_deps.py')
    shell_utils.Bash(['python', roll_deps_script,
                      '-c', os.curdir,
                      '--git_hash', self._got_revision,
                      '--git_path', GIT,
                      '--skia_git_path', os.path.join('third_party', 'skia'),
                      '--verbose',
                      '--delete_branches'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DEPSRoll))

#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the DEPS roll automation script."""


import os
import sys

from build_step import BuildStep


GIT = 'git.bat' if os.name == 'nt' else 'git'


class DEPSRoll(BuildStep):
  """BuildStep which creates and uploads a DEPS roll CL and sets off trybots."""

  def _Run(self):
    skia_path = os.path.join('third_party', 'skia')
    roll_deps_path = os.path.join(skia_path, 'tools')
    sys.path.append(roll_deps_path)
    import roll_deps

    class Options(object):
      verbose = True
      delete_branches = True
      search_depth = 1
      chromium_path = os.curdir
      git_path = GIT
      skip_cl_upload = False
      bots = ','.join(roll_deps.DEFAULT_BOTS_LIST)
      skia_git_path = skia_path
    options = Options()
    config = roll_deps.DepsRollConfig(options)

    revision, _ = \
        roll_deps.revision_and_hash_from_partial(config, self._got_revision)

    deps_issue, whitespace_issue = \
        roll_deps.roll_deps(config, revision, self._got_revision)

    print 'Deps roll', deps_issue
    print 'Control', whitespace_issue


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DEPSRoll))

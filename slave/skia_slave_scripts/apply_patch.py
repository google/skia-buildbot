#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Apply a diff to patch a Skia checkout. """


from ast import literal_eval
from build_step import BuildStep, BuildStepFailure
from utils import shell_utils
import os
import sys
import tempfile


class ApplyPatch(BuildStep):
  def _Run(self):
    if self._args['patch'] == 'None':
      raise BuildStepFailure('No patch given!')
    if self._args['patch_root'] == 'None':
      raise BuildStepFailure('Problem with patch: no root specified!')

    # patch is a tuple of the form (int, str), where patch[0] is the "level" of
    # the patch and patch[1] is the diff.
    patch = literal_eval(self._args['patch'].decode())
    print 'Patch level: %d' % patch[0]
    print 'Diff:'
    print patch[1]

    patch_file = tempfile.NamedTemporaryFile()
    patch_file.write(patch[1])
    patch_file.flush()
    print 'Saved patch to %s' % patch_file.name

    # Make sure we're always in the right place to apply the patch.
    patch_root = self._args['patch_root'].replace('/', os.path.sep)
    os.chdir(os.pardir)
    if patch_root != 'svn':
      os.chdir(patch_root)
    try:
      shell_utils.Bash(['patch', '-p%d' % patch[0], '-i', patch_file.name])
    finally:
      # Closing a NamedTemporaryFile also deletes it.
      patch_file.close()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ApplyPatch))
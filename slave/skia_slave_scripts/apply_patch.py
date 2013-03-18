#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Apply a diff to patch a Skia checkout. """


from ast import literal_eval
from build_step import BuildStep, BuildStepFailure
from utils import shell_utils
import os
import shutil
import sys
import tempfile
import urllib


if os.name == 'nt':
  SVN = 'svn.bat'
else:
  SVN = 'svn'

WIN_PATCH = os.path.abspath(os.path.join(os.pardir, os.pardir, os.pardir,
                                         os.pardir, os.pardir, 'GnuWin32',
                                         'patch.exe'))


class ApplyPatch(BuildStep):
  def _Run(self):
    if self._args['patch'] == 'None':
      raise BuildStepFailure('No patch given!')
    if self._args['patch_root'] == 'None':
      raise BuildStepFailure('Problem with patch: no root specified!')

    # patch is a tuple of the form (int, str), where patch[0] is the "level" of
    # the patch and patch[1] is the diff.
    patch = literal_eval(self._args['patch'].decode())
    patch_level = patch[0]
    patch_url = urllib.quote(patch[1], safe="%/:=&?~+!$,;'@()*[]")
    print 'Patch level: %d' % patch[0]
    print 'Diff file URL:'
    print patch_url

    # Write the patch file into a temporary directory. Unfortunately, temporary
    # files created by the tempfile module don't behave properly on Windows, so
    # we create a temporary directory and write the file inside it.
    temp_dir = tempfile.mkdtemp()
    try:
      patch_file_name = os.path.join(temp_dir, 'skiabot_patch')
      patch_file = open(patch_file_name, 'w')
      try:
        # TODO(borenet): Create an svn_utils module and use it instead.  It
        # would be nice to find a way to share
        # http://skia.googlecode.com/svn/trunk/tools/svn.py
        patch_contents = shell_utils.Bash([SVN, 'cat', patch_url], echo=False)
        patch_file.write(patch_contents.read())
      finally:
        patch_file.close()
      print 'Saved patch to %s' % patch_file.name
  
      # On Windows, use the patch.exe included in the checkout
      if os.name == 'nt':
        patcher = WIN_PATCH
      else:
        patcher = 'patch'
  
      # Make sure we're always in the right place to apply the patch.
      patch_root = self._args['patch_root'].replace('/', os.path.sep)
      os.chdir(os.pardir)
      if patch_root != 'svn':
        os.chdir(patch_root)
  
      shell_utils.Bash([patcher, '-p%d' % patch_level, '-i', patch_file.name])
    finally:
      shutil.rmtree(temp_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ApplyPatch))
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
import urllib2


if os.name == 'nt':
  SVN = 'svn.bat'
else:
  SVN = 'svn'

WIN_PATCH = os.path.abspath(os.path.join(os.path.dirname(__file__), os.pardir,
                                         os.pardir, 'third_party', 'GnuWin32',
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
    # Assume that the patch level that was passed in is incorrect, since that
    # is most often the case.  Instead use 1, because patches from git checkouts
    # have an extra level.
    patch_level = 1
    patch_url = urllib.quote(patch[1], safe="%/:=&?~+!$,;'@()*[]")
    print 'Patch level: %d' % patch_level
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
        if 'svn' in patch_url:
          # TODO(borenet): Create an svn_utils module and use it instead.  It
          # would be nice to find a way to share
          # http://skia.googlecode.com/svn/trunk/tools/svn.py
          patch_contents = shell_utils.Bash([SVN, 'cat', patch_url], echo=False)
        else:
          patch_contents = urllib2.urlopen(patch_url).read()
        patch_file.write(patch_contents)
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
      if patch_root != 'svn' and patch_root != '':
        os.chdir(patch_root)

      shell_utils.Bash([patcher, '-p%d' % patch_level, '-i', patch_file.name,
                        '-r', '-'])

    finally:
      shutil.rmtree(temp_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ApplyPatch))

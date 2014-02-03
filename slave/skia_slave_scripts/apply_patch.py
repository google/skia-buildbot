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
  GIT = 'git.bat'
  SVN = 'svn.bat'
else:
  GIT = 'git'
  SVN = 'svn'


class ApplyPatch(BuildStep):
  def _Run(self):
    if self._args['patch'] == 'None':
      raise BuildStepFailure('No patch given!')

    # patch is a tuple of the form (int, str), where patch[0] is the "level" of
    # the patch and patch[1] is the URL of the diff.
    patch_level, encoded_patch_url = literal_eval(self._args['patch'].decode())
    patch_url = urllib.quote(encoded_patch_url, safe="%/:=&?~+!$,;'@()*[]")
    print 'Patch level: %d' % patch_level
    print 'Diff file URL:'
    print patch_url

    # Write the patch file into a temporary directory. Unfortunately, temporary
    # files created by the tempfile module don't behave properly on Windows, so
    # we create a temporary directory and write the file inside it.
    temp_dir = tempfile.mkdtemp()
    try:
      patch_file_name = os.path.join(temp_dir, 'skiabot_patch')
      patch_file = open(patch_file_name, 'wb')
      try:
        if 'svn' in patch_url:
          # TODO(borenet): Create an svn_utils module and use it instead.  It
          # would be nice to find a way to share
          # https://skia.googlesource.com/skia/+/master/tools/svn.py
          patch_contents = shell_utils.run([SVN, 'cat', patch_url], echo=False)
        else:
          patch_contents = urllib2.urlopen(patch_url).read()
        patch_file.write(patch_contents)
      finally:
        patch_file.close()
      print 'Saved patch to %s' % patch_file.name

      def get_patch_cmd(level, patch_filename):
        return [GIT, 'apply', '-p%d' % level, '-v', '--ignore-space-change',
                '--ignore-whitespace', patch_filename]

      try:
        # First, check that the patch can be applied at the given level.
        shell_utils.run(get_patch_cmd(patch_level, patch_file.name) +
                        ['--check'])
      except shell_utils.CommandFailedException as e:
        # If the patch can't be applied at the requested level, try 0 or 1,
        # depending on what we just tried.
        print e
        patch_level = (patch_level + 1) % 2
        print 'Trying patch level %d instead...' % patch_level
      shell_utils.run(get_patch_cmd(patch_level, patch_file.name))

    finally:
      shutil.rmtree(temp_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ApplyPatch))
